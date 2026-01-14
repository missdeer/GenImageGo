package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"genimage/auth"
	"genimage/model"

	"gorm.io/gorm"
)

var (
	ErrNotMember          = errors.New("user not org member")
	ErrInsufficientPoints = errors.New("insufficient org points")
	ErrSuperAdminTarget   = errors.New("target user is super admin")
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
	ID   uint             `json:"id"`
	Name string           `json:"name"`
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
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Points int    `json:"points"`
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
		items[i] = orgListItem{ID: o.ID, Name: o.Name, Points: o.Points}
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

func (h *AdminHandler) AllocateUserPoints(w http.ResponseWriter, r *http.Request) {
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
		UserID         uint `json:"user_id"`
		Points         int  `json:"points"`
		OrganizationID uint `json:"organization_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.Points <= 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "积分必须大于0"})
		return
	}

	db := h.authService.DB()

	// 先检查目标用户是否存在
	var targetUser model.User
	if err := db.First(&targetUser, req.UserID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
		return
	}

	// 非超级管理员不能操作超级管理员
	if targetUser.Type == model.UserTypeSuperAdmin && !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作超级管理员"})
		return
	}

	// 基本权限检查
	if !h.canManageUser(permissions, req.UserID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该用户"})
		return
	}

	// 超级管理员直接增加用户积分，不扣组织积分
	if permissions.IsSuperAdmin {
		result := db.Model(&model.User{}).Where("id = ?", req.UserID).
			UpdateColumn("points", gorm.Expr("points + ?", req.Points))

		if result.Error != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
			return
		}
		if result.RowsAffected == 0 {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户不存在"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "points_added": req.Points})
		return
	}

	// 组织管理员需要从组织积分划拨
	if req.OrganizationID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织ID不能为空"})
		return
	}

	// 验证管理员是否有权管理该组织
	var hasPermission bool
	for _, org := range permissions.ManagedOrganizations {
		if org.ID == req.OrganizationID {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作该组织"})
		return
	}

	// 使用事务保证原子性，所有检查和更新都在事务内
	err = db.Transaction(func(tx *gorm.DB) error {
		// 验证用户存在
		var user model.User
		if err := tx.First(&user, req.UserID).Error; err != nil {
			return err
		}
		if user.Type == model.UserTypeSuperAdmin {
			return ErrSuperAdminTarget
		}

		// 验证用户是该组织成员
		var membership model.Membership
		if err := tx.Where("user_id = ? AND organization_id = ?", req.UserID, req.OrganizationID).
			First(&membership).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return ErrNotMember
			}
			return err
		}

		// 使用条件更新确保积分充足（适用于所有数据库）
		result := tx.Model(&model.Organization{}).
			Where("id = ? AND points >= ?", req.OrganizationID, req.Points).
			UpdateColumn("points", gorm.Expr("points - ?", req.Points))

		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 检查是组织不存在还是积分不足
			var org model.Organization
			if err := tx.First(&org, req.OrganizationID).Error; err != nil {
				return err
			}
			return ErrInsufficientPoints
		}

		// 增加用户积分
		result = tx.Model(&model.User{}).Where("id = ?", req.UserID).
			UpdateColumn("points", gorm.Expr("points + ?", req.Points))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "用户不是该组织成员"})
		} else if errors.Is(err, ErrSuperAdminTarget) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "无权限操作超级管理员"})
		} else if errors.Is(err, ErrInsufficientPoints) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织积分不足"})
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "用户或组织不存在"})
		} else {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "操作失败"})
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "points_added": req.Points})
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
	OrganizationID uint             `json:"organization_id"`
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

type orgFullListItem struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Points      int       `json:"points"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type listOrgsFullResponse struct {
	Organizations []orgFullListItem `json:"organizations"`
	Total         int64             `json:"total"`
	Page          int               `json:"page"`
	PageSize      int               `json:"page_size"`
	TotalPages    int               `json:"total_pages"`
}

func (h *AdminHandler) ListOrgsForManagement(w http.ResponseWriter, r *http.Request) {
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

	db := h.authService.DB()

	var allowedOrgIDs []uint
	if !permissions.IsSuperAdmin {
		for _, org := range permissions.ManagedOrganizations {
			allowedOrgIDs = append(allowedOrgIDs, org.ID)
		}
		if len(allowedOrgIDs) == 0 {
			writeJSON(w, http.StatusOK, listOrgsFullResponse{
				Organizations: []orgFullListItem{},
				Total:         0,
				Page:          page,
				PageSize:      pageSize,
				TotalPages:    0,
			})
			return
		}
	}

	query := db.Model(&model.Organization{})

	if !permissions.IsSuperAdmin {
		query = query.Where("id IN ?", allowedOrgIDs)
	}

	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	offset := (page - 1) * pageSize
	var orgs []model.Organization
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&orgs).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "查询失败"})
		return
	}

	orgIDs := make([]uint, len(orgs))
	for i, o := range orgs {
		orgIDs[i] = o.ID
	}

	memberCounts := make(map[uint]int64)
	if len(orgIDs) > 0 {
		type countResult struct {
			OrganizationID uint
			Count          int64
		}
		var results []countResult
		db.Model(&model.Membership{}).
			Select("organization_id, count(*) as count").
			Where("organization_id IN ?", orgIDs).
			Group("organization_id").
			Scan(&results)
		for _, r := range results {
			memberCounts[r.OrganizationID] = r.Count
		}
	}

	items := make([]orgFullListItem, len(orgs))
	for i, o := range orgs {
		items[i] = orgFullListItem{
			ID:          o.ID,
			Name:        o.Name,
			Points:      o.Points,
			MemberCount: memberCounts[o.ID],
			CreatedAt:   o.CreatedAt,
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, listOrgsFullResponse{
		Organizations: items,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
		TotalPages:    totalPages,
	})
}

func (h *AdminHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
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
	if !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可创建组织"})
		return
	}

	var req struct {
		Name   string `json:"name"`
		Points int    `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织名称不能为空"})
		return
	}

	if req.Points < 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "积分不能为负数"})
		return
	}

	org := model.Organization{
		Name:   name,
		Points: req.Points,
	}

	db := h.authService.DB()
	if err := db.Create(&org).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织名称已存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "创建失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"organization": map[string]interface{}{
			"id":     org.ID,
			"name":   org.Name,
			"points": org.Points,
		},
	})
}

func (h *AdminHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
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
	if !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可解散组织"})
		return
	}

	var req struct {
		OrganizationID uint `json:"organization_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.OrganizationID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织ID无效"})
		return
	}

	db := h.authService.DB()
	var org model.Organization
	if err := db.First(&org, req.OrganizationID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "组织不存在"})
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("organization_id = ?", req.OrganizationID).Delete(&model.Membership{}).Error; err != nil {
			return err
		}
		return tx.Delete(&org).Error
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "解散失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

func (h *AdminHandler) UpdateOrganizationPoints(w http.ResponseWriter, r *http.Request) {
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
	if !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可修改组织积分"})
		return
	}

	var req struct {
		OrganizationID uint `json:"organization_id"`
		Points         int  `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.OrganizationID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织ID无效"})
		return
	}

	if req.Points < 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "积分不能为负数"})
		return
	}

	db := h.authService.DB()
	var org model.Organization
	if err := db.First(&org, req.OrganizationID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "组织不存在"})
		return
	}

	if err := db.Model(&model.Organization{}).Where("id = ?", req.OrganizationID).Update("points", req.Points).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "更新失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "points": req.Points})
}

func (h *AdminHandler) UpdateOrganizationName(w http.ResponseWriter, r *http.Request) {
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
	if !permissions.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "仅超级管理员可修改组织名称"})
		return
	}

	var req struct {
		OrganizationID uint   `json:"organization_id"`
		Name           string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "无效的请求格式"})
		return
	}

	if req.OrganizationID == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织ID无效"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "组织名称不能为空"})
		return
	}

	db := h.authService.DB()
	var org model.Organization
	if err := db.First(&org, req.OrganizationID).Error; err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "组织不存在"})
		return
	}

	if err := db.Model(&model.Organization{}).Where("id = ?", req.OrganizationID).Update("name", name).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
			writeJSON(w, http.StatusConflict, errorResponse{Error: "组织名称已存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "更新失败"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "name": name})
}
