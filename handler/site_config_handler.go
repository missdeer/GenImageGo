package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"genimage/auth"
	"genimage/model"
	"genimage/siteconfig"
)

type SiteConfigHandler struct {
	authService   *auth.Service
	configService *siteconfig.Service
}

func NewSiteConfigHandler(authService *auth.Service, configService *siteconfig.Service) *SiteConfigHandler {
	return &SiteConfigHandler{
		authService:   authService,
		configService: configService,
	}
}

func (h *SiteConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	if user.Type != model.UserTypeSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可访问"})
		return
	}

	configs := h.configService.GetAll()
	if configs == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "加载配置失败"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
	})
}

func (h *SiteConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	if user.Type != model.UserTypeSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可访问"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Configs []siteconfig.ConfigUpdate `json:"configs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if len(req.Configs) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "配置项不能为空"})
		return
	}

	if err := h.configService.SetBatch(req.Configs); err != nil {
		log.Printf("siteconfig update failed: %v", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"updated": len(req.Configs),
	})
}

func (h *SiteConfigHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetConfig(w, r)
	case http.MethodPost:
		h.UpdateConfig(w, r)
	default:
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}
