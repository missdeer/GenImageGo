package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"genimage/auth"
	"genimage/model"

	"gorm.io/gorm"
)

type AdminHandler struct {
	authService *auth.Service
}

func NewAdminHandler(authService *auth.Service) *AdminHandler {
	return &AdminHandler{authService: authService}
}

type userListItem struct {
	ID            uint                `json:"id"`
	Email         string              `json:"email"`
	Type          model.UserType      `json:"type"`
	EmailVerified bool                `json:"email_verified"`
	Disabled      bool                `json:"disabled"`
	Points        int                 `json:"points"`
	CreatedAt     time.Time           `json:"created_at"`
	Organizations []orgMembershipInfo `json:"organizations,omitempty"`
}

type orgMembershipInfo struct {
	ID   uint            `json:"id"`
	Name string          `json:"name"`
	Role model.MemberRole `json:"role"`
}

type listUsersResponse struct {
	Users      []userListItem `json:"users"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
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

	keyword := r.URL.Query().Get("keyword")
	orgIDStr := r.URL.Query().Get("organization_id")
	var filterOrgID uint
	if orgIDStr != "" {
		if id, err := strconv.ParseUint(orgIDStr, 10, 32); err == nil {
			filterOrgID = uint(id)
		}
	}

	db := h.authService.DB()

	var allowedOrgIDs []uint
	if !permissions.IsSuperAdmin {
		for _, org := range permissions.ManagedOrganizations {
			allowedOrgIDs = append(allowedOrgIDs, org.ID)
		}
		if filterOrgID > 0 && !contains(allowedOrgIDs, filterOrgID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限访问该组织"})
			return
		}
	}

	query := db.Model(&model.User{})

	if !permissions.IsSuperAdmin {
		query = query.Joins("JOIN memberships ON users.id = memberships.user_id AND memberships.deleted_at IS NULL").
			Where("memberships.organization_id IN ?", allowedOrgIDs).
			Distinct()
	}

	if filterOrgID > 0 {
		if permissions.IsSuperAdmin {
			query = query.Joins("JOIN memberships ON users.id = memberships.user_id AND memberships.deleted_at IS NULL").
				Where("memberships.organization_id = ?", filterOrgID).
				Distinct()
		} else {
			query = query.Where("memberships.organization_id = ?", filterOrgID)
		}
	}

	if keyword != "" {
		query = query.Where("users.email LIKE ?", "%"+keyword+"%")
	}

	var total int64
	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var users []model.User
	if err := query.Order("users.created_at DESC, users.id DESC").
		Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	userIDs := make([]uint, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	var memberships []model.Membership
	if len(userIDs) > 0 {
		if err := db.Preload("Organization").Where("user_id IN ?", userIDs).Find(&memberships).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
			return
		}
	}

	membershipMap := make(map[uint][]orgMembershipInfo)
	for _, m := range memberships {
		if m.Organization == nil {
			continue
		}
		if !permissions.IsSuperAdmin && !contains(allowedOrgIDs, m.OrganizationID) {
			continue
		}
		membershipMap[m.UserID] = append(membershipMap[m.UserID], orgMembershipInfo{
			ID:   m.OrganizationID,
			Name: m.Organization.Name,
			Role: m.Role,
		})
	}

	items := make([]userListItem, len(users))
	for i, u := range users {
		items[i] = userListItem{
			ID:            u.ID,
			Email:         u.Email,
			Type:          u.Type,
			EmailVerified: u.EmailVerified,
			Disabled:      u.Disabled,
			Points:        u.Points,
			CreatedAt:     u.CreatedAt,
			Organizations: membershipMap[u.ID],
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, listUsersResponse{
		Users:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func contains(slice []uint, val uint) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

type orgListItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func (h *AdminHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
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

	db := h.authService.DB()
	var orgs []model.Organization

	if permissions.IsSuperAdmin {
		if err := db.Order("name").Find(&orgs).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
			return
		}
	} else {
		var orgIDs []uint
		for _, org := range permissions.ManagedOrganizations {
			orgIDs = append(orgIDs, org.ID)
		}
		if len(orgIDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{"organizations": []orgListItem{}})
			return
		}
		if err := db.Where("id IN ?", orgIDs).Order("name").Find(&orgs).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
			return
		}
	}

	items := make([]orgListItem, len(orgs))
	for i, o := range orgs {
		items[i] = orgListItem{ID: o.ID, Name: o.Name}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"organizations": items})
}

func (h *AdminHandler) canManageUser(permissions *auth.AdminPermissions, targetUserID uint) bool {
	if permissions.IsSuperAdmin {
		return true
	}
	var orgIDs []uint
	for _, org := range permissions.ManagedOrganizations {
		orgIDs = append(orgIDs, org.ID)
	}
	if len(orgIDs) == 0 {
		return false
	}
	db := h.authService.DB()
	var count int64
	db.Model(&model.Membership{}).Where("user_id = ? AND organization_id IN ?", targetUserID, orgIDs).Count(&count)
	return count > 0
}

func (h *AdminHandler) ToggleUserDisabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	var req struct {
		UserID   uint `json:"user_id"`
		Disabled bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.UserID == user.ID {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "不能禁用自己"})
		return
	}

	db := h.authService.DB()
	var targetUser model.User
	if err := db.First(&targetUser, req.UserID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
		return
	}

	if targetUser.Type == model.UserTypeSuperAdmin && !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作超级管理员"})
		return
	}

	if !h.canManageUser(permissions, req.UserID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
		return
	}

	if err := db.Model(&model.User{}).Where("id = ?", req.UserID).Update("disabled", req.Disabled).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		return
	}

	if req.Disabled {
		db.Where("user_id = ?", req.UserID).Delete(&model.Session{})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "disabled": req.Disabled})
}

func (h *AdminHandler) UpdateUserPoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	var req struct {
		UserID uint `json:"user_id"`
		Points int  `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.Points < 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "积分不能为负数"})
		return
	}

	db := h.authService.DB()
	var targetUser model.User
	if err := db.First(&targetUser, req.UserID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
		return
	}

	if !h.canManageUser(permissions, req.UserID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
		return
	}

	if err := db.Model(&model.User{}).Where("id = ?", req.UserID).Update("points", req.Points).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "points": req.Points})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	var req struct {
		UserID uint `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.UserID == user.ID {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "不能删除自己"})
		return
	}

	db := h.authService.DB()
	var targetUser model.User
	if err := db.First(&targetUser, req.UserID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
		return
	}

	if targetUser.Type == model.UserTypeSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "不能删除超级管理员"})
		return
	}

	if !h.canManageUser(permissions, req.UserID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", req.UserID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", req.UserID).Delete(&model.Membership{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, req.UserID).Error
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "删除失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

type membershipUpdate struct {
	OrganizationID uint            `json:"organization_id"`
	Role           model.MemberRole `json:"role"`
}

func (h *AdminHandler) UpdateUserMemberships(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	var req struct {
		UserID      uint               `json:"user_id"`
		Memberships []membershipUpdate `json:"memberships"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	db := h.authService.DB()
	var targetUser model.User
	if err := db.First(&targetUser, req.UserID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
		return
	}

	if targetUser.Type == model.UserTypeSuperAdmin && !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作超级管理员"})
		return
	}

	if !h.canManageUser(permissions, req.UserID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
		return
	}

	var allowedOrgIDs []uint
	if permissions.IsSuperAdmin {
		var orgs []model.Organization
		if err := db.Find(&orgs).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
			return
		}
		for _, org := range orgs {
			allowedOrgIDs = append(allowedOrgIDs, org.ID)
		}
	} else {
		for _, org := range permissions.ManagedOrganizations {
			allowedOrgIDs = append(allowedOrgIDs, org.ID)
		}
	}

	if len(allowedOrgIDs) == 0 {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无可管理的组织"})
		return
	}

	for _, m := range req.Memberships {
		if !contains(allowedOrgIDs, m.OrganizationID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该组织"})
			return
		}
		if m.Role != model.MemberRoleMember && m.Role != model.MemberRoleAdmin {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的角色值"})
			return
		}
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("user_id = ? AND organization_id IN ?", req.UserID, allowedOrgIDs).
			Delete(&model.Membership{}).Error; err != nil {
			return err
		}

		for _, m := range req.Memberships {
			membership := model.Membership{
				UserID:         req.UserID,
				OrganizationID: m.OrganizationID,
				Role:           m.Role,
			}
			if err := tx.Create(&membership).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}
