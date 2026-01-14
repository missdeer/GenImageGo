package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"genimage/auth"
	"genimage/model"
	"genimage/points"
)

func handleEnhancePrompt(h *Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("enhance-prompt request from %s", r.RemoteAddr)

	// Step 1: Validate request BEFORE deducting points
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	reqPrompt := string(body)

	systemPrompt := h.config.EnhancePromptText
	if systemPrompt == "" {
		log.Printf("enhance-prompt system prompt not configured")
		http.Error(w, "system prompt not configured", http.StatusInternalServerError)
		return
	}

	apiKey := h.config.APIKey
	if apiKey == "" {
		apiKey = h.config.DefaultAPIKey
	}
	if apiKey == "" {
		log.Printf("enhance-prompt missing API key")
		http.Error(w, "missing API key", http.StatusBadRequest)
		return
	}

	modelName := h.config.TextModel
	if modelName == "" {
		modelName = h.config.DefaultTextModel
	}

	apiService := h.config.APIService
	if apiService == "" {
		apiService = h.config.DefaultAPIService
	}

	// Step 2: Deduct points (after validation passes)
	var user *model.User
	requiredPoints := h.config.EnhancePromptPoints
	var deductRecord *model.PointTransaction

	if requiredPoints > 0 && h.db != nil {
		user = auth.GetUserFromContext(r.Context())
		if user == nil {
			writeJSONError(w, http.StatusUnauthorized, "用户未登录")
			return
		}

		idempotencyKey := r.Header.Get("Idempotency-Key")
		record, err := points.DeductUserPoints(h.db, points.DeductUserPointsParams{
			UserID:      user.ID,
			Amount:      requiredPoints,
			Reason:      model.PointReasonEnhancePrompt,
			OperationID: idempotencyKey,
		})
		if err != nil {
			if err == points.ErrInsufficientPoints {
				var currentUser model.User
				h.db.Select("points").First(&currentUser, user.ID)
				log.Printf("enhance-prompt insufficient points: user=%d current=%d required=%d", user.ID, currentUser.Points, requiredPoints)
				writeJSONError(w, http.StatusPaymentRequired, fmt.Sprintf("积分不足，当前积分: %d，需要积分: %d", currentUser.Points, requiredPoints))
				return
			}
			log.Printf("enhance-prompt deduct points failed: user=%d err=%v", user.ID, err)
			writeJSONError(w, http.StatusInternalServerError, "扣除积分失败")
			return
		}
		deductRecord = record
		log.Printf("enhance-prompt deduct points: user=%d points=%d record=%d", user.ID, requiredPoints, record.ID)
	}

	// Step 3: Make upstream API request
	var text string
	switch apiService {
	case "openai":
		text, err = h.enhancePromptViaOpenAI(r, reqPrompt, systemPrompt, modelName, apiKey, start)
	default:
		text, err = h.enhancePromptViaGemini(r, reqPrompt, systemPrompt, modelName, apiKey, start)
	}

	if err != nil {
		if deductRecord != nil && user != nil {
			refundPoints(h.db, user.ID, requiredPoints, deductRecord.ID, "enhance-prompt")
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}

func (h *Handler) enhancePromptViaGemini(r *http.Request, reqPrompt, systemPrompt, model, apiKey string, start time.Time) (string, error) {
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURL = h.config.DefaultBaseURL
	}

	geminiReq := geminiRequest{
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: reqPrompt}},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			ResponseMIMEType: "application/json",
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		log.Printf("enhance-prompt failed to marshal request: %v", err)
		return "", err
	}

	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("enhance-prompt upstream error: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	log.Printf("enhance-prompt upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("enhance-prompt failed to read response: %v", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("enhance-prompt upstream error response: %s", string(respBody))
		return "", fmt.Errorf("upstream error: %s", string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		log.Printf("enhance-prompt failed to parse response: %v", err)
		return "", err
	}

	if geminiResp.Error != nil {
		log.Printf("enhance-prompt API error: %s", geminiResp.Error.Message)
		return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		log.Printf("enhance-prompt empty response from API")
		return "", fmt.Errorf("empty response from API")
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
	log.Printf("enhance-prompt result text: %s", text)
	return cleanResponseText(text), nil
}

func (h *Handler) enhancePromptViaOpenAI(r *http.Request, reqPrompt, systemPrompt, model, apiKey string, start time.Time) (string, error) {
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURL = h.config.DefaultBaseURL
	}

	openaiReq := openaiChatRequest{
		Model: model,
		Messages: []openaiChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: reqPrompt},
		},
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		log.Printf("enhance-prompt failed to marshal request: %v", err)
		return "", err
	}

	apiURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(baseURL, "/"))

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("enhance-prompt upstream error: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	log.Printf("enhance-prompt upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("enhance-prompt failed to read response: %v", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("enhance-prompt upstream error response: %s", string(respBody))
		return "", fmt.Errorf("upstream error: %s", string(respBody))
	}

	var openaiResp openaiChatResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		log.Printf("enhance-prompt failed to parse response: %v", err)
		return "", err
	}

	if openaiResp.Error != nil {
		log.Printf("enhance-prompt API error: %s", openaiResp.Error.Message)
		return "", fmt.Errorf("API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		log.Printf("enhance-prompt empty response from API")
		return "", fmt.Errorf("empty response from API")
	}

	text := openaiResp.Choices[0].Message.Content
	log.Printf("enhance-prompt result text: %s", text)
	return cleanResponseText(text), nil
}

func cleanResponseText(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	return text
}
