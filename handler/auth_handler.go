package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"genimage/auth"
	"genimage/mail"
	"genimage/model"
)

type AuthHandler struct {
	authService *auth.Service
	mailService *mail.Service
	baseWebURL  string
}

func NewAuthHandler(authService *auth.Service, mailService *mail.Service, baseWebURL string) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		mailService: mailService,
		baseWebURL:  baseWebURL,
	}
}

type registerRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	ReferralCode string `json:"referral_code"`
}

type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type userResponse struct {
	ID                   uint                       `json:"id"`
	Email                string                     `json:"email"`
	EmailVerified        bool                       `json:"email_verified"`
	Type                 model.UserType             `json:"type"`
	Points               int                        `json:"points"`
	CanManageUsers       bool                       `json:"can_manage_users"`
	IsSuperAdmin         bool                       `json:"is_super_admin"`
	ManagedOrganizations []auth.ManagedOrganization `json:"managed_organizations,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.mailService == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "邮件服务未配置，无法注册"})
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	user, session, err := h.authService.Register(req.Email, req.Password, req.ReferralCode)
	if err != nil {
		status := http.StatusBadRequest
		if err == auth.ErrEmailExists {
			status = http.StatusConflict
		} else if err == auth.ErrReferralCodeInvalid {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}

	verifyToken, err := h.authService.CreateEmailVerificationToken(user.ID)
	if err != nil {
		h.authService.CleanupFailedRegistration(user.ID)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "创建验证令牌失败"})
		return
	}

	verifyLink := fmt.Sprintf("%s/api/auth/verify-email?token=%s", h.baseWebURL, verifyToken.Token)
	if err := h.mailService.SendVerificationEmail(req.Email, verifyLink); err != nil {
		h.authService.CleanupFailedRegistration(user.ID)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "发送验证邮件失败，请稍后重试"})
		return
	}

	auth.SetSessionCookie(w, session.Token)
	writeJSON(w, http.StatusCreated, userResponse{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Type:          user.Type,
		Points:        user.Points,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	user, session, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
		return
	}

	auth.SetSessionCookieWithExpiry(w, session.Token, req.RememberMe)
	writeJSON(w, http.StatusOK, userResponse{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Type:          user.Type,
		Points:        user.Points,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		h.authService.Logout(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	permissions, err := h.authService.GetAdminPermissions(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	resp := userResponse{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Type:          user.Type,
		Points:        user.Points,
	}

	if permissions != nil {
		resp.CanManageUsers = permissions.CanManageUsers
		resp.IsSuperAdmin = permissions.IsSuperAdmin
		resp.ManagedOrganizations = permissions.ManagedOrganizations
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	resetToken, err := h.authService.CreatePasswordResetToken(req.Email)
	if err != nil {
		if err == auth.ErrEmailNotFound {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if h.mailService != nil {
		resetLink := fmt.Sprintf("%s/reset-password.html?token=%s", h.baseWebURL, resetToken.Token)
		if err := h.mailService.SendPasswordResetEmail(req.Email, resetLink); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "发送邮件失败，请稍后重试"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "缺少 token 参数"})
		return
	}

	user, err := h.authService.ValidatePasswordResetToken(token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid": true,
		"email": user.Email,
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if err := h.authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	if user.EmailVerified {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: auth.ErrEmailAlreadyVerified.Error()})
		return
	}

	if h.mailService == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "邮件服务未配置"})
		return
	}

	verifyToken, err := h.authService.CreateEmailVerificationToken(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "创建验证令牌失败"})
		return
	}

	verifyLink := fmt.Sprintf("%s/api/auth/verify-email?token=%s", h.baseWebURL, verifyToken.Token)
	if err := h.mailService.SendVerificationEmail(user.Email, verifyLink); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "发送邮件失败，请稍后重试"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/?error=missing_token", http.StatusFound)
		return
	}

	if err := h.authService.VerifyEmail(token); err != nil {
		http.Redirect(w, r, "/?error=invalid_token", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *AuthHandler) Profile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	if _, err := h.authService.EnsureUserReferralCode(user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "获取推荐码失败"})
		return
	}

	profile, err := h.authService.GetUserProfile(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "获取用户信息失败"})
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if err := h.authService.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "密码修改成功，请重新登录"})
}
