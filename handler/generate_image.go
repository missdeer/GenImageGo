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

	"gorm.io/gorm"
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
	modelName := h.config.Model
	if modelName == "" {
		modelSource = "default"
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
		http.Error(w, "missing API key", http.StatusBadRequest)
		return
	}

	// Step 2: Deduct points (after validation passes)
	var user *model.User
	requiredPoints := h.config.ImageGenerationPoints
	pointsDeducted := false

	if requiredPoints > 0 && h.db != nil {
		user = auth.GetUserFromContext(r.Context())
		if user == nil {
			writeJSONError(w, http.StatusUnauthorized, "用户未登录")
			return
		}

		result := h.db.Model(&model.User{}).
			Where("id = ? AND points >= ?", user.ID, requiredPoints).
			UpdateColumn("points", gorm.Expr("points - ?", requiredPoints))
		if result.Error != nil {
			log.Printf("generate-image deduct points failed: user=%d err=%v", user.ID, result.Error)
			writeJSONError(w, http.StatusInternalServerError, "扣除积分失败")
			return
		}
		if result.RowsAffected == 0 {
			var currentUser model.User
			h.db.Select("points").First(&currentUser, user.ID)
			log.Printf("generate-image insufficient points: user=%d current=%d required=%d", user.ID, currentUser.Points, requiredPoints)
			writeJSONError(w, http.StatusPaymentRequired, fmt.Sprintf("积分不足，当前积分: %d，需要积分: %d", currentUser.Points, requiredPoints))
			return
		}
		pointsDeducted = true
		log.Printf("generate-image deduct points: user=%d points=%d", user.ID, requiredPoints)
	}

	// Step 3: Make upstream API request
	baseURLSource := h.config.BaseURLSource
	baseURL := h.config.BaseURL
	if baseURL == "" {
		baseURLSource = "default"
		baseURL = h.config.DefaultBaseURL
	}
	log.Printf("generate-image upstream model=%s (source=%s) baseURL=%s (source=%s) apiKeySource=%s keyLen=%d", modelName, modelSource, baseURL, baseURLSource, apiKeySource, len(apiKey))
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(modelName),
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
		if pointsDeducted && user != nil {
			refundPoints(h.db, user.ID, requiredPoints, "generate-image")
		}
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if pointsDeducted && user != nil {
			refundPoints(h.db, user.ID, requiredPoints, "generate-image")
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
