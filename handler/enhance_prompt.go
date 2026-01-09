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
)

func handleEnhancePrompt(h *Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("enhance-prompt request from %s", r.RemoteAddr)

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
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURL = h.config.DefaultBaseURL
	}
	if baseURL == "" {
		baseURL = h.config.DefaultGeminiURL
	}

	model := h.config.TextModel
	if model == "" {
		model = h.config.DefaultTextModel
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("enhance-prompt upstream error: %v", err)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("enhance-prompt upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("enhance-prompt failed to read response: %v", err)
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("enhance-prompt upstream error response: %s", string(respBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		log.Printf("enhance-prompt failed to parse response: %v", err)
		http.Error(w, "failed to parse upstream response", http.StatusBadGateway)
		return
	}

	if geminiResp.Error != nil {
		log.Printf("enhance-prompt API error: %s", geminiResp.Error.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(respBody)
		return
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		log.Printf("enhance-prompt empty response from API")
		http.Error(w, "empty response from API", http.StatusBadGateway)
		return
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
	log.Printf("enhance-prompt result text: %s", text)
	// Trim whitespace (spaces, newlines, carriage returns, tabs)
	text = strings.TrimSpace(text)
	// Remove markdown JSON code fences if present
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}
