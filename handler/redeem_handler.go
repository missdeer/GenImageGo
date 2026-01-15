package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"genimage/auth"
	"genimage/model"
	"genimage/redeem"
)

type RedeemHandler struct {
	authService *auth.Service
}

func NewRedeemHandler(authService *auth.Service) *RedeemHandler {
	return &RedeemHandler{authService: authService}
}

type generateCodesRequest struct {
	Points int    `json:"points"`
	Count  int    `json:"count"`
	Note   string `json:"note"`
}

type redeemCodeItem struct {
	ID     uint   `json:"id"`
	Code   string `json:"code"`
	Points int    `json:"points"`
}

func (h *RedeemHandler) GenerateCodes(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可生成兑换码"})
		return
	}

	var req generateCodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.Points <= 0 || req.Points > 100000 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "积分值必须在1-100000之间"})
		return
	}
	if req.Count <= 0 || req.Count > 1000 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "生成数量必须在1-1000之间"})
		return
	}
	if req.Points*req.Count > 1000000 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "单次生成总额不能超过1000000积分"})
		return
	}
	if len(req.Note) > 200 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "备注长度不能超过200字符"})
		return
	}

	codes, err := redeem.GenerateCodes(h.authService.DB(), redeem.GenerateParams{
		Points:    req.Points,
		Count:     req.Count,
		Note:      req.Note,
		CreatorID: user.ID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "生成兑换码失败"})
		return
	}

	items := make([]redeemCodeItem, len(codes))
	for i, c := range codes {
		items[i] = redeemCodeItem{
			ID:     c.ID,
			Code:   c.Code,
			Points: c.Points,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"codes": items,
		"count": len(items),
	})
}

type listCodesItem struct {
	ID                uint       `json:"id"`
	Code              string     `json:"code"`
	Points            int        `json:"points"`
	Status            int        `json:"status"`
	StatusText        string     `json:"status_text"`
	CreatedAt         time.Time  `json:"created_at"`
	CreatorID         uint       `json:"creator_id"`
	Note              string     `json:"note,omitempty"`
	RedeemedAt        *time.Time `json:"redeemed_at,omitempty"`
	RedeemedByUserID  *uint      `json:"redeemed_by_user_id,omitempty"`
	RedeemedByEmail   string     `json:"redeemed_by_email,omitempty"`
}

func (h *RedeemHandler) ListCodes(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可查看兑换码"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	statusStr := r.URL.Query().Get("status")
	var statusFilter *int
	if statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil {
			statusFilter = &s
		}
	}

	// Additional filters
	pointsStr := r.URL.Query().Get("points")
	createdStart := r.URL.Query().Get("created_start")
	createdEnd := r.URL.Query().Get("created_end")
	redeemedStart := r.URL.Query().Get("redeemed_start")
	redeemedEnd := r.URL.Query().Get("redeemed_end")
	redeemedByEmail := r.URL.Query().Get("redeemed_by")

	db := h.authService.DB()
	query := db.Model(&model.RedeemCode{})

	if statusFilter != nil {
		query = query.Where("status = ?", *statusFilter)
	}

	if pointsStr != "" {
		if pts, err := strconv.Atoi(pointsStr); err == nil && pts > 0 {
			query = query.Where("points = ?", pts)
		}
	}

	if createdStart != "" {
		if t, err := time.Parse("2006-01-02", createdStart); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if createdEnd != "" {
		if t, err := time.Parse("2006-01-02", createdEnd); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	if redeemedStart != "" {
		if t, err := time.Parse("2006-01-02", redeemedStart); err == nil {
			query = query.Where("redeemed_at >= ?", t)
		}
	}
	if redeemedEnd != "" {
		if t, err := time.Parse("2006-01-02", redeemedEnd); err == nil {
			query = query.Where("redeemed_at < ?", t.AddDate(0, 0, 1))
		}
	}

	if redeemedByEmail != "" {
		var userIDs []uint
		escapedEmail := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(redeemedByEmail, "\\", "\\\\"), "%", "\\%"), "_", "\\_")
		db.Model(&model.User{}).Where("email LIKE ? ESCAPE '\\\\'", "%"+escapedEmail+"%").Pluck("id", &userIDs)
		if len(userIDs) > 0 {
			query = query.Where("redeemed_by_user_id IN ?", userIDs)
		} else {
			query = query.Where("1 = 0") // No match
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var codes []model.RedeemCode
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&codes).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	// Collect redeemed user IDs
	var userIDs []uint
	for _, c := range codes {
		if c.RedeemedByUserID != nil {
			userIDs = append(userIDs, *c.RedeemedByUserID)
		}
	}

	// Query user emails
	userEmailMap := make(map[uint]string)
	if len(userIDs) > 0 {
		var users []model.User
		db.Select("id, email").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			userEmailMap[u.ID] = u.Email
		}
	}

	items := make([]listCodesItem, len(codes))
	for i, c := range codes {
		var redeemedByEmail string
		if c.RedeemedByUserID != nil {
			redeemedByEmail = userEmailMap[*c.RedeemedByUserID]
		}
		items[i] = listCodesItem{
			ID:               c.ID,
			Code:             c.Code,
			Points:           c.Points,
			Status:           int(c.Status),
			StatusText:       c.Status.String(),
			CreatedAt:        c.CreatedAt,
			CreatorID:        c.CreatorID,
			Note:             c.Note,
			RedeemedAt:       c.RedeemedAt,
			RedeemedByUserID: c.RedeemedByUserID,
			RedeemedByEmail:  redeemedByEmail,
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"codes":       items,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

type disableCodeRequest struct {
	CodeID uint `json:"code_id"`
}

func (h *RedeemHandler) DisableCode(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可禁用兑换码"})
		return
	}

	var req disableCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.CodeID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "兑换码ID不能为空"})
		return
	}

	if err := redeem.DisableCode(h.authService.DB(), req.CodeID); err != nil {
		if err == redeem.ErrCodeNotFound {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "兑换码不存在或已被使用/禁用"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type enableCodeRequest struct {
	CodeID uint `json:"code_id"`
}

func (h *RedeemHandler) EnableCode(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可启用兑换码"})
		return
	}

	var req enableCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.CodeID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "兑换码ID不能为空"})
		return
	}

	if err := redeem.EnableCode(h.authService.DB(), req.CodeID); err != nil {
		if err == redeem.ErrCodeNotFound {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "兑换码不存在或状态不是已禁用"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type redeemRequest struct {
	Code        string `json:"code"`
	TargetType  string `json:"target_type"`
	TargetOrgID uint   `json:"target_org_id"`
}

func (h *RedeemHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
		return
	}

	if !user.EmailVerified {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "请先验证邮箱"})
		return
	}

	var req redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "兑换码不能为空"})
		return
	}

	var targetType model.RedeemCodeTargetType
	if req.TargetType == "org" {
		targetType = model.RedeemCodeTargetOrg
		if req.TargetOrgID == 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "请选择目标组织"})
			return
		}

		isAdmin, err := redeem.IsOrgAdmin(h.authService.DB(), user.ID, req.TargetOrgID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
			return
		}
		if !isAdmin {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "您不是该组织的管理员"})
			return
		}
	} else {
		targetType = model.RedeemCodeTargetUser
	}

	result, err := redeem.RedeemCode(h.authService.DB(), redeem.RedeemParams{
		Code:        req.Code,
		UserID:      user.ID,
		TargetType:  targetType,
		TargetOrgID: req.TargetOrgID,
	})
	if err != nil {
		switch err {
		case redeem.ErrCodeNotFound:
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "兑换码不存在"})
		case redeem.ErrCodeAlreadyUsed:
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "兑换码已被使用"})
		case redeem.ErrCodeDisabled:
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "兑换码已禁用"})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "兑换失败"})
		}
		return
	}

	targetTypeStr := "user"
	if result.TargetType == model.RedeemCodeTargetOrg {
		targetTypeStr = "org"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"points":      result.Points,
		"new_balance": result.NewBalance,
		"target_type": targetTypeStr,
	})
}

func (h *RedeemHandler) GetManagedOrgs(w http.ResponseWriter, r *http.Request) {
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
	var memberships []model.Membership
	if err := db.Preload("Organization").
		Where("user_id = ? AND role = ?", user.ID, model.MemberRoleAdmin).
		Find(&memberships).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	type orgItem struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}

	orgs := make([]orgItem, 0, len(memberships))
	for _, m := range memberships {
		if m.Organization != nil {
			orgs = append(orgs, orgItem{
				ID:   m.Organization.ID,
				Name: m.Organization.Name,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"organizations": orgs,
	})
}
