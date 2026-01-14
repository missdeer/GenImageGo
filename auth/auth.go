package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"genimage/model"
	"genimage/points"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	bcryptCost                = 10
	sessionTokenLen           = 32
	resetTokenLen             = 32
	verificationTokenLen      = 32
	referralCodeLen           = 8
	referralBonus             = 100
	sessionDuration           = 7 * 24 * time.Hour
	resetTokenDuration        = 1 * time.Hour
	verificationTokenDuration = 1 * time.Hour
	minPasswordLen            = 6
)

var (
	ErrEmailInvalid             = errors.New("邮箱格式不正确")
	ErrEmailExists              = errors.New("该邮箱已被注册")
	ErrEmailNotFound            = errors.New("该邮箱未注册")
	ErrPasswordTooShort         = errors.New("密码长度至少为 6 个字符")
	ErrPasswordTooWeak          = errors.New("密码必须包含大写字母、小写字母和数字")
	ErrInvalidCredentials       = errors.New("邮箱或密码错误")
	ErrUserDisabled             = errors.New("账号已被禁用")
	ErrSessionExpired           = errors.New("会话已过期")
	ErrSessionNotFound          = errors.New("会话不存在")
	ErrResetTokenInvalid        = errors.New("重置链接无效或已过期")
	ErrResetTokenUsed           = errors.New("重置链接已被使用")
	ErrVerificationTokenInvalid = errors.New("验证链接无效或已过期")
	ErrVerificationTokenUsed    = errors.New("验证链接已被使用")
	ErrEmailAlreadyVerified     = errors.New("邮箱已验证")
	ErrReferralCodeInvalid      = errors.New("推荐码无效")
	ErrCurrentPasswordWrong     = errors.New("当前密码错误")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const referralCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateReferralCode() (string, error) {
	b := make([]byte, referralCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = referralCodeChars[int(b[i])%len(referralCodeChars)]
	}
	return string(b), nil
}

func validatePasswordComplexity(password string) bool {
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// isUniqueConstraintError checks if err is a unique constraint violation across different databases
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// Check for GORM's translated error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	// SQLite: "UNIQUE constraint failed"
	// MySQL: "Duplicate entry"
	// PostgreSQL: "duplicate key value violates unique constraint"
	return strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "duplicate entry") ||
		strings.Contains(errStr, "duplicate key")
}

// isEmailUniqueError checks if the unique constraint error is for the email field
func isEmailUniqueError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for email field name in error message
	// Different databases may include column name, constraint name, or table.column
	return strings.Contains(errStr, "email") ||
		strings.Contains(errStr, "users.email") ||
		strings.Contains(errStr, "idx_users_email")
}

type Service struct {
	db               *gorm.DB
	dailyLoginPoints int
}

func NewService(db *gorm.DB, dailyLoginPoints int) *Service {
	return &Service{db: db, dailyLoginPoints: dailyLoginPoints}
}

func (s *Service) Register(email, password, referralCode string) (*model.User, *model.Session, error) {
	if !emailRegex.MatchString(email) {
		return nil, nil, ErrEmailInvalid
	}
	if len(password) < minPasswordLen {
		return nil, nil, ErrPasswordTooShort
	}
	if !validatePasswordComplexity(password) {
		return nil, nil, ErrPasswordTooWeak
	}

	// Normalize referral code to uppercase
	referralCode = strings.ToUpper(strings.TrimSpace(referralCode))

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, nil, err
	}

	var user *model.User
	var session *model.Session

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Check email existence within transaction
		var existing model.User
		if err := tx.Where("email = ?", email).First(&existing).Error; err == nil {
			return ErrEmailExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Validate referrer inside transaction
		var referrerID *uint
		if referralCode != "" {
			var referrer model.User
			if err := tx.Where("referral_code = ?", referralCode).First(&referrer).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrReferralCodeInvalid
				}
				return err
			}
			referrerID = &referrer.ID
		}

		// Generate unique referral code with retry, including handling unique constraint violations
		var newReferralCode string
		var createErr error
		for attempt := 0; attempt < 10; attempt++ {
			code, err := generateReferralCode()
			if err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.User{}).Where("referral_code = ?", code).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				newReferralCode = code
			} else {
				continue
			}

			user = &model.User{
				Email:        email,
				PasswordHash: string(hash),
				ReferralCode: newReferralCode,
				ReferredBy:   referrerID,
			}
			createErr = tx.Create(user).Error
			if createErr == nil {
				break
			}

			// Check if it's a unique constraint violation
			if isUniqueConstraintError(createErr) {
				// Distinguish between email and referral_code collision
				if isEmailUniqueError(createErr) {
					return ErrEmailExists
				}
				// referral_code collision - retry with new code
				user = nil
				continue
			}
			return createErr
		}
		if user == nil {
			if createErr != nil {
				return createErr
			}
			return errors.New("无法生成推荐码")
		}

		// Award referral bonus within the same transaction
		if referrerID != nil {
			referralOpID := fmt.Sprintf("referral:%d", user.ID)
			_, err := points.AddUserPointsTx(tx, points.AddUserPointsParams{
				UserID:      *referrerID,
				Amount:      referralBonus,
				Reason:      model.PointReasonReferralBonus,
				OperationID: referralOpID,
			})
			if err != nil {
				return err
			}
			_, err = points.AddUserPointsTx(tx, points.AddUserPointsParams{
				UserID:      user.ID,
				Amount:      referralBonus,
				Reason:      model.PointReasonReferredBonus,
				OperationID: referralOpID,
			})
			if err != nil {
				return err
			}
			user.Points = referralBonus
		}

		// Create session within transaction
		tokenBytes := make([]byte, sessionTokenLen)
		if _, err := rand.Read(tokenBytes); err != nil {
			return err
		}

		session = &model.Session{
			Token:     hex.EncodeToString(tokenBytes),
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(sessionDuration),
		}

		return tx.Create(session).Error
	})

	if err != nil {
		return nil, nil, err
	}

	return user, session, nil
}

func (s *Service) CleanupFailedRegistration(userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.EmailVerificationToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.PasswordResetToken{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&model.User{}, userID).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) Login(email, password string) (*model.User, *model.Session, error) {
	var user model.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if user.Disabled {
		return nil, nil, ErrUserDisabled
	}

	if s.dailyLoginPoints > 0 && user.EmailVerified && user.Type == model.UserTypeNormal {
		now := time.Now().UTC()
		dateStr := now.Format("2006-01-02")
		record, err := points.AwardDailyLoginPoints(s.db, points.AwardDailyLoginParams{
			UserID: user.ID,
			Amount: s.dailyLoginPoints,
			Date:   dateStr,
		})
		if err == nil && record != nil {
			user.Points += s.dailyLoginPoints
			user.LastPointsDate = &now
		}
	}

	session, err := s.createSession(user.ID)
	if err != nil {
		return nil, nil, err
	}

	return &user, session, nil
}

func (s *Service) Logout(token string) error {
	return s.db.Where("token = ?", token).Delete(&model.Session{}).Error
}

func (s *Service) ValidateSession(token string) (*model.User, error) {
	var session model.Session
	if err := s.db.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, ErrSessionNotFound
	}

	if time.Now().After(session.ExpiresAt) {
		s.db.Delete(&session)
		return nil, ErrSessionExpired
	}

	var user model.User
	if err := s.db.First(&user, session.UserID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) CleanExpiredSessions() error {
	return s.db.Where("expires_at < ?", time.Now()).Delete(&model.Session{}).Error
}

func (s *Service) createSession(userID uint) (*model.Session, error) {
	tokenBytes := make([]byte, sessionTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}

	session := &model.Session{
		Token:     hex.EncodeToString(tokenBytes),
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

func (s *Service) CreatePasswordResetToken(email string) (*model.PasswordResetToken, error) {
	if !emailRegex.MatchString(email) {
		return nil, ErrEmailInvalid
	}

	var user model.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, ErrEmailNotFound
	}

	s.db.Where("user_id = ? AND used = ?", user.ID, false).Delete(&model.PasswordResetToken{})

	tokenBytes := make([]byte, resetTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}

	resetToken := &model.PasswordResetToken{
		Token:     hex.EncodeToString(tokenBytes),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(resetTokenDuration),
		Used:      false,
	}

	if err := s.db.Create(resetToken).Error; err != nil {
		return nil, err
	}

	return resetToken, nil
}

func (s *Service) ValidatePasswordResetToken(token string) (*model.User, error) {
	var resetToken model.PasswordResetToken
	if err := s.db.Where("token = ?", token).First(&resetToken).Error; err != nil {
		return nil, ErrResetTokenInvalid
	}

	if resetToken.Used {
		return nil, ErrResetTokenUsed
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return nil, ErrResetTokenInvalid
	}

	var user model.User
	if err := s.db.First(&user, resetToken.UserID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) ResetPassword(token, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return ErrPasswordTooShort
	}
	if !validatePasswordComplexity(newPassword) {
		return ErrPasswordTooWeak
	}

	var resetToken model.PasswordResetToken
	if err := s.db.Where("token = ?", token).First(&resetToken).Error; err != nil {
		return ErrResetTokenInvalid
	}

	if resetToken.Used {
		return ErrResetTokenUsed
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return ErrResetTokenInvalid
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	if err := s.db.Model(&model.User{}).Where("id = ?", resetToken.UserID).Update("password_hash", string(hash)).Error; err != nil {
		return err
	}

	s.db.Model(&resetToken).Update("used", true)

	s.db.Where("user_id = ?", resetToken.UserID).Delete(&model.Session{})

	return nil
}

func (s *Service) CleanExpiredResetTokens() error {
	return s.db.Where("expires_at < ? OR used = ?", time.Now(), true).Delete(&model.PasswordResetToken{}).Error
}

func (s *Service) CreateEmailVerificationToken(userID uint) (*model.EmailVerificationToken, error) {
	if err := s.db.Where("user_id = ? AND used = ?", userID, false).Delete(&model.EmailVerificationToken{}).Error; err != nil {
		return nil, err
	}

	tokenBytes := make([]byte, verificationTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}

	token := &model.EmailVerificationToken{
		Token:     hex.EncodeToString(tokenBytes),
		UserID:    userID,
		ExpiresAt: time.Now().Add(verificationTokenDuration),
		Used:      false,
	}

	if err := s.db.Create(token).Error; err != nil {
		return nil, err
	}

	return token, nil
}

func (s *Service) VerifyEmail(token string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var verifyToken model.EmailVerificationToken
		if err := tx.Where("token = ?", token).First(&verifyToken).Error; err != nil {
			return ErrVerificationTokenInvalid
		}

		if verifyToken.Used {
			return ErrVerificationTokenUsed
		}

		if time.Now().After(verifyToken.ExpiresAt) {
			return ErrVerificationTokenInvalid
		}

		result := tx.Model(&model.EmailVerificationToken{}).
			Where("id = ? AND used = ?", verifyToken.ID, false).
			Update("used", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVerificationTokenUsed
		}

		if err := tx.Model(&model.User{}).Where("id = ?", verifyToken.UserID).Update("email_verified", true).Error; err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) CleanExpiredVerificationTokens() error {
	return s.db.Where("expires_at < ? OR used = ?", time.Now(), true).Delete(&model.EmailVerificationToken{}).Error
}

type ManagedOrganization struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type AdminPermissions struct {
	CanManageUsers       bool                  `json:"can_manage_users"`
	IsSuperAdmin         bool                  `json:"is_super_admin"`
	ManagedOrganizations []ManagedOrganization `json:"managed_organizations,omitempty"`
}

func (s *Service) GetAdminPermissions(userID uint) (*AdminPermissions, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	if user.Type == model.UserTypeSuperAdmin {
		return &AdminPermissions{
			CanManageUsers:       true,
			IsSuperAdmin:         true,
			ManagedOrganizations: nil,
		}, nil
	}

	var memberships []model.Membership
	if err := s.db.Preload("Organization").Where("user_id = ? AND role = ?", userID, model.MemberRoleAdmin).Find(&memberships).Error; err != nil {
		return nil, err
	}

	if len(memberships) == 0 {
		return &AdminPermissions{
			CanManageUsers:       false,
			IsSuperAdmin:         false,
			ManagedOrganizations: nil,
		}, nil
	}

	orgs := make([]ManagedOrganization, 0, len(memberships))
	for _, m := range memberships {
		if m.Organization != nil {
			orgs = append(orgs, ManagedOrganization{
				ID:   m.Organization.ID,
				Name: m.Organization.Name,
			})
		}
	}

	return &AdminPermissions{
		CanManageUsers:       true,
		IsSuperAdmin:         false,
		ManagedOrganizations: orgs,
	}, nil
}

func (s *Service) DB() *gorm.DB {
	return s.db
}

func (s *Service) ChangePassword(userID uint, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return ErrPasswordTooShort
	}
	if !validatePasswordComplexity(newPassword) {
		return ErrPasswordTooWeak
	}

	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordWrong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	if err := s.db.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		return err
	}

	s.db.Where("user_id = ?", userID).Delete(&model.Session{})

	return nil
}

type UserOrganization struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type UserProfile struct {
	ID            uint               `json:"id"`
	Email         string             `json:"email"`
	EmailVerified bool               `json:"email_verified"`
	Points        int                `json:"points"`
	ReferralCode  string             `json:"referral_code"`
	Type          model.UserType     `json:"type"`
	CreatedAt     string             `json:"created_at"`
	Organizations []UserOrganization `json:"organizations"`
}

func (s *Service) GetUserProfile(userID uint) (*UserProfile, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	var memberships []model.Membership
	if err := s.db.Preload("Organization").Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
		return nil, err
	}

	orgs := make([]UserOrganization, 0, len(memberships))
	for _, m := range memberships {
		if m.Organization != nil {
			orgs = append(orgs, UserOrganization{
				ID:   m.Organization.ID,
				Name: m.Organization.Name,
				Role: m.Role.String(),
			})
		}
	}

	return &UserProfile{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Points:        user.Points,
		ReferralCode:  user.ReferralCode,
		Type:          user.Type,
		CreatedAt:     user.CreatedAt.Format("2006-01-02"),
		Organizations: orgs,
	}, nil
}

func (s *Service) EnsureUserReferralCode(userID uint) (string, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return "", err
	}

	if user.ReferralCode != "" {
		return user.ReferralCode, nil
	}

	var newCode string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Re-check within transaction
		var u model.User
		if err := tx.First(&u, userID).Error; err != nil {
			return err
		}
		if u.ReferralCode != "" {
			newCode = u.ReferralCode
			return nil
		}

		// Try multiple times in case of unique constraint violations
		var updateErr error
		for attempt := 0; attempt < 10; attempt++ {
			code, err := generateReferralCode()
			if err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.User{}).Where("referral_code = ?", code).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}

			// Conditional update to prevent overwrite in case of concurrent calls
			result := tx.Model(&model.User{}).Where("id = ? AND (referral_code = '' OR referral_code IS NULL)", userID).Update("referral_code", code)
			updateErr = result.Error
			if updateErr != nil {
				// Check if unique constraint violation, retry with new code
				if isUniqueConstraintError(updateErr) {
					continue
				}
				return updateErr
			}
			if result.RowsAffected == 0 {
				// Another goroutine already set it, re-fetch
				if err := tx.First(&u, userID).Error; err != nil {
					return err
				}
				newCode = u.ReferralCode
				return nil
			}
			newCode = code
			return nil
		}

		if updateErr != nil {
			return updateErr
		}
		return errors.New("无法生成推荐码")
	})

	if err != nil {
		return "", err
	}

	return newCode, nil
}
