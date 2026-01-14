package handler

import (
	"net/http"
	"strconv"
	"time"

	"genimage/auth"
	"genimage/model"
)

type PointsHandler struct {
	authService *auth.Service
}

func NewPointsHandler(authService *auth.Service) *PointsHandler {
	return &PointsHandler{authService: authService}
}

type pointTransactionItem struct {
	ID           uint              `json:"id"`
	CreatedAt    time.Time         `json:"created_at"`
	Amount       int               `json:"amount"`
	Reason       model.PointReason `json:"reason"`
	ReasonText   string            `json:"reason_text"`
	Description  string            `json:"description,omitempty"`
	BalanceAfter int               `json:"balance_after"`
	RelatedOrgID *uint             `json:"related_org_id,omitempty"`
}

type listPointsHistoryResponse struct {
	Transactions []pointTransactionItem `json:"transactions"`
	Total        int64                  `json:"total"`
	Page         int                    `json:"page"`
	PageSize     int                    `json:"page_size"`
	TotalPages   int                    `json:"total_pages"`
}

var reasonTextMap = map[model.PointReason]string{
	model.PointReasonDailyLogin:    "每日登录",
	model.PointReasonReferralBonus: "推荐奖励",
	model.PointReasonReferredBonus: "被推荐奖励",
	model.PointReasonImageGen:      "生成图片",
	model.PointReasonEnhancePrompt: "提示词优化",
	model.PointReasonRefund:        "积分退还",
	model.PointReasonAdminGrant:    "管理员充值",
	model.PointReasonOrgAllocation: "组织划拨",
	model.PointReasonOrgInitial:    "组织初始积分",
	model.PointReasonOrgAdjust:     "组织积分调整",
	model.PointReasonOrgAllocOut:   "组织划拨支出",
}

func (h *PointsHandler) UserPointsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "未授权访问"})
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

	reason := r.URL.Query().Get("reason")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	db := h.authService.DB()
	query := db.Model(&model.PointTransaction{}).Where("type = ? AND user_id = ?", model.PointTransactionTypeUser, user.ID)

	if reason != "" {
		query = query.Where("reason = ?", reason)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var transactions []model.PointTransaction
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&transactions).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	items := make([]pointTransactionItem, len(transactions))
	for i, t := range transactions {
		items[i] = pointTransactionItem{
			ID:           t.ID,
			CreatedAt:    t.CreatedAt,
			Amount:       t.Amount,
			Reason:       t.Reason,
			ReasonText:   reasonTextMap[t.Reason],
			Description:  t.Description,
			BalanceAfter: t.BalanceAfter,
			RelatedOrgID: t.RelatedOrgID,
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, listPointsHistoryResponse{
		Transactions: items,
		Total:        total,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
	})
}

func (h *PointsHandler) AdminUserPointsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
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
	if !permissions.CanManageUsers {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限访问"})
		return
	}

	targetUserIDStr := r.URL.Query().Get("user_id")
	if targetUserIDStr == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "缺少 user_id 参数"})
		return
	}
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的 user_id"})
		return
	}

	db := h.authService.DB()

	if !permissions.IsSuperAdmin {
		var orgIDs []uint
		for _, org := range permissions.ManagedOrganizations {
			orgIDs = append(orgIDs, org.ID)
		}
		if len(orgIDs) == 0 {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
			return
		}
		var count int64
		db.Model(&model.Membership{}).Where("user_id = ? AND organization_id IN ?", targetUserID, orgIDs).Count(&count)
		if count == 0 {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
			return
		}
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

	reason := r.URL.Query().Get("reason")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	query := db.Model(&model.PointTransaction{}).Where("type = ? AND user_id = ?", model.PointTransactionTypeUser, targetUserID)

	if reason != "" {
		query = query.Where("reason = ?", reason)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var transactions []model.PointTransaction
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&transactions).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	items := make([]pointTransactionItem, len(transactions))
	for i, t := range transactions {
		items[i] = pointTransactionItem{
			ID:           t.ID,
			CreatedAt:    t.CreatedAt,
			Amount:       t.Amount,
			Reason:       t.Reason,
			ReasonText:   reasonTextMap[t.Reason],
			Description:  t.Description,
			BalanceAfter: t.BalanceAfter,
			RelatedOrgID: t.RelatedOrgID,
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, listPointsHistoryResponse{
		Transactions: items,
		Total:        total,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
	})
}

type orgPointTransactionItem struct {
	ID            uint              `json:"id"`
	CreatedAt     time.Time         `json:"created_at"`
	Amount        int               `json:"amount"`
	Reason        model.PointReason `json:"reason"`
	ReasonText    string            `json:"reason_text"`
	Description   string            `json:"description,omitempty"`
	BalanceAfter  int               `json:"balance_after"`
	RelatedUserID *uint             `json:"related_user_id,omitempty"`
}

type listOrgPointsHistoryResponse struct {
	Transactions []orgPointTransactionItem `json:"transactions"`
	Total        int64                     `json:"total"`
	Page         int                       `json:"page"`
	PageSize     int                       `json:"page_size"`
	TotalPages   int                       `json:"total_pages"`
}

func (h *PointsHandler) AdminOrgPointsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
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
	if !permissions.CanManageUsers {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限访问"})
		return
	}

	orgIDStr := r.URL.Query().Get("organization_id")
	if orgIDStr == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "缺少 organization_id 参数"})
		return
	}
	orgID, err := strconv.ParseUint(orgIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的 organization_id"})
		return
	}

	if !permissions.IsSuperAdmin {
		var hasPermission bool
		for _, org := range permissions.ManagedOrganizations {
			if org.ID == uint(orgID) {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该组织"})
			return
		}
	}

	db := h.authService.DB()

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

	reason := r.URL.Query().Get("reason")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	query := db.Model(&model.PointTransaction{}).Where("type = ? AND organization_id = ?", model.PointTransactionTypeOrg, orgID)

	if reason != "" {
		query = query.Where("reason = ?", reason)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var transactions []model.PointTransaction
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&transactions).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	items := make([]orgPointTransactionItem, len(transactions))
	for i, t := range transactions {
		items[i] = orgPointTransactionItem{
			ID:            t.ID,
			CreatedAt:     t.CreatedAt,
			Amount:        t.Amount,
			Reason:        t.Reason,
			ReasonText:    reasonTextMap[t.Reason],
			Description:   t.Description,
			BalanceAfter:  t.BalanceAfter,
			RelatedUserID: t.RelatedUserID,
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, listOrgPointsHistoryResponse{
		Transactions: items,
		Total:        total,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
	})
}
