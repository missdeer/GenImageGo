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

func handleGenerateImage(h *Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("generate-image request from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}
	log.Printf("generate-image payload: %s", string(body))
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	modelSource := h.config.ModelSource
	model := h.config.Model
	if model == "" {
		modelSource = "default"
		model = h.config.DefaultModel
	}
	apiKeySource := h.config.APIKeySource
	apiKey := h.config.APIKey
	if apiKey == "" {
		apiKeySource = "default"
		apiKey = h.config.DefaultAPIKey
	}
	if apiKey == "" {
		log.Printf("generate-image missing API key (source=%s)", apiKeySource)
		http.Error(w, "missing API key", http.StatusBadRequest)
		return
	}
	baseURLSource := h.config.BaseURLSource
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURLSource = "default"
		baseURL = h.config.DefaultBaseURL
	}
	if baseURL == "" {
		baseURLSource = "builtin"
		baseURL = h.config.DefaultGeminiURL
	}
	log.Printf("generate-image upstream model=%s (source=%s) baseURL=%s (source=%s) apiKeySource=%s keyLen=%d", model, modelSource, baseURL, baseURLSource, apiKeySource, len(apiKey))
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("generate-image upstream error: %v", err)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("generate-image upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
