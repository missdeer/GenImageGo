package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"genimage/auth"
	"genimage/model"
	"genimage/points"
)

func handleGenerateImage(h *Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("generate-image request from %s", r.RemoteAddr)

	// Step 1: Validate request BEFORE deducting points
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty request body")
		return
	}

	var req geminiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	modelName := h.config.Model
	if modelName == "" {
		modelName = h.config.DefaultModel
	}
	apiKeySource := h.config.APIKeySource
	apiKey := h.config.APIKey
	if apiKey == "" {
		apiKeySource = "default"
		apiKey = h.config.DefaultAPIKey
	}
	if apiKey == "" {
		log.Printf("generate-image missing API key (source=%s)", apiKeySource)
		writeJSONError(w, http.StatusBadRequest, "missing API key")
		return
	}

	apiService := h.config.APIService
	if apiService == "" {
		apiService = h.config.DefaultAPIService
	}

	// Step 2: Deduct points (after validation passes)
	var user *model.User
	requiredPoints := 20
	if h.config.SiteConfigService != nil {
		requiredPoints = h.config.SiteConfigService.GetInt(model.ConfigKeyImageGenerationPoints, 20)
	}
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
			Reason:      model.PointReasonImageGen,
			OperationID: idempotencyKey,
		})
		if err != nil {
			if err == points.ErrInsufficientPoints {
				var currentUser model.User
				h.db.Select("points").First(&currentUser, user.ID)
				log.Printf("generate-image insufficient points: user=%d current=%d required=%d", user.ID, currentUser.Points, requiredPoints)
				writeJSONError(w, http.StatusPaymentRequired, fmt.Sprintf("积分不足，当前积分: %d，需要积分: %d", currentUser.Points, requiredPoints))
				return
			}
			log.Printf("generate-image deduct points failed: user=%d err=%v", user.ID, err)
			writeJSONError(w, http.StatusInternalServerError, "扣除积分失败")
			return
		}
		deductRecord = record
		log.Printf("generate-image deduct points: user=%d points=%d record=%d", user.ID, requiredPoints, record.ID)
	}

	// Step 3: Make upstream API request
	var respBody []byte
	switch apiService {
	case "openai":
		respBody, err = h.generateImageViaOpenAI(r, &req, modelName, apiKey, start)
	default:
		respBody, err = h.generateImageViaGemini(r, body, modelName, apiKey, start)
	}

	if err != nil {
		if deductRecord != nil && user != nil {
			refundPoints(h.db, user.ID, requiredPoints, deductRecord.ID, "generate-image")
		}
		log.Printf("generate-image upstream error: %v", err)
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (h *Handler) generateImageViaGemini(r *http.Request, body []byte, model, apiKey string, start time.Time) ([]byte, error) {
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURL = h.config.DefaultBaseURL
	}

	log.Printf("generate-image upstream via Gemini model=%s baseURL=%s", model, baseURL)

	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("generate-image upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream error (status=%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (h *Handler) generateImageViaOpenAI(r *http.Request, req *geminiRequest, model, apiKey string, start time.Time) ([]byte, error) {
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURL = h.config.DefaultBaseURL
	}

	log.Printf("generate-image upstream via OpenAI model=%s baseURL=%s", model, baseURL)

	// Extract prompt and images from Gemini request format
	prompt, images := extractPromptAndImages(req)
	if prompt == "" {
		prompt = "Generate image"
	}

	// Build OpenAI request with multimodal content
	var content []interface{}
	content = append(content, map[string]string{"type": "text", "text": prompt})
	for _, img := range images {
		mimeType := img.MIMEType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:%s;base64,%s", mimeType, img.Data),
			},
		})
	}

	openaiReq := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(baseURL, "/"))

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create upstream request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("generate-image upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream error (status=%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse OpenAI response and convert to Gemini format
	var openaiResp openaiChatResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	responseContent := openaiResp.Choices[0].Message.Content
	return convertToGeminiResponse(responseContent)
}

type imageData struct {
	MIMEType string
	Data     string
}

func extractPromptAndImages(req *geminiRequest) (string, []imageData) {
	var prompts []string
	var images []imageData

	for _, content := range req.Contents {
		for _, part := range content.Parts {
			if part.Text != "" {
				prompts = append(prompts, part.Text)
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				images = append(images, imageData{
					MIMEType: part.InlineData.MIMEType,
					Data:     part.InlineData.Data,
				})
			}
		}
	}

	return strings.Join(prompts, "\n"), images
}

func convertToGeminiResponse(content string) ([]byte, error) {
	var parts []geminiResponsePart

	// Try to extract image from markdown format: ![image](data:image/xxx;base64,...)
	imageBytes, mimeType, err := parseImageFromMarkdown(content)
	if err == nil && imageBytes != nil {
		b64Data := base64.StdEncoding.EncodeToString(imageBytes)
		parts = append(parts, geminiResponsePart{
			InlineData: &geminiResponseInlineData{
				MIMEType: mimeType,
				Data:     b64Data,
			},
		})
		// Remove the markdown image from text
		content = removeMarkdownImages(content)
		content = strings.TrimSpace(content)
	}

	// Add text content if any
	if content != "" {
		parts = append(parts, geminiResponsePart{
			Text: content,
		})
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no image or text in response")
	}

	geminiResp := geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: &geminiResponseContent{
					Role:  "model",
					Parts: parts,
				},
			},
		},
	}

	return json.Marshal(geminiResp)
}

var imageMarkdownRe = regexp.MustCompile(`!\[[^\]]*\]\(data:image/([\w.+-]+);base64,([A-Za-z0-9+/=]+)\)`)

func parseImageFromMarkdown(content string) ([]byte, string, error) {
	matches := imageMarkdownRe.FindStringSubmatch(content)
	if len(matches) >= 3 {
		decoded, err := base64.StdEncoding.DecodeString(matches[2])
		if err != nil {
			return nil, "", err
		}
		return decoded, "image/" + matches[1], nil
	}
	return nil, "", fmt.Errorf("no image found")
}

func removeMarkdownImages(content string) string {
	return imageMarkdownRe.ReplaceAllString(content, "")
}
