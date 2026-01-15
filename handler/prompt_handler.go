package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"genimage/auth"
	"genimage/model"

	"gorm.io/gorm"
)

type PromptHandler struct {
	authService *auth.Service
}

func NewPromptHandler(authService *auth.Service) *PromptHandler {
	return &PromptHandler{authService: authService}
}

type createPromptRequest struct {
	ClientID string `json:"client_id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

type updatePromptRequest struct {
	ID      uint   `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type deletePromptRequest struct {
	ID uint `json:"id"`
}

func (h *PromptHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	db := h.authService.DB()
	var prompts []model.CustomPrompt
	if err := db.Where("user_id = ?", user.ID).Order("created_at DESC, id DESC").Find(&prompts).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"prompts": prompts,
	})
}

func (h *PromptHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	var req createPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "client_id 不能为空"})
		return
	}
	if !model.IsValidUUID(req.ClientID) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "client_id 格式无效"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "标题不能为空"})
		return
	}
	if utf8.RuneCountInString(req.Title) > 100 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "标题不能超过100个字符"})
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "内容不能为空"})
		return
	}
	if utf8.RuneCountInString(req.Content) > 10000 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "内容不能超过10000个字符"})
		return
	}

	db := h.authService.DB()

	var existing model.CustomPrompt
	if err := db.Where("user_id = ? AND client_id = ?", user.ID, req.ClientID).First(&existing).Error; err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
			"prompt": existing,
		})
		return
	}

	prompt := model.CustomPrompt{
		UserID:   user.ID,
		ClientID: req.ClientID,
		Title:    req.Title,
		Content:  req.Content,
	}

	if err := db.Create(&prompt).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "创建失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"prompt": prompt,
	})
}

func (h *PromptHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	var req updatePromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.ID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "ID不能为空"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "标题不能为空"})
		return
	}
	if utf8.RuneCountInString(req.Title) > 100 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "标题不能超过100个字符"})
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "内容不能为空"})
		return
	}
	if utf8.RuneCountInString(req.Content) > 10000 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "内容不能超过10000个字符"})
		return
	}

	db := h.authService.DB()

	var prompt model.CustomPrompt
	if err := db.Where("id = ? AND user_id = ?", req.ID, user.ID).First(&prompt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "提示词不存在"})
		} else {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		}
		return
	}

	if err := db.Model(&prompt).Updates(map[string]interface{}{
		"title":   req.Title,
		"content": req.Content,
	}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "更新失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *PromptHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	var req deletePromptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.ID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "ID不能为空"})
		return
	}

	db := h.authService.DB()

	result := db.Where("id = ? AND user_id = ?", req.ID, user.ID).Delete(&model.CustomPrompt{})
	if result.Error != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "删除失败"})
		return
	}
	if result.RowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "提示词不存在"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
