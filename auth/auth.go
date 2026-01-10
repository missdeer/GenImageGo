package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"genimage/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	bcryptCost                = 10
	sessionTokenLen           = 32
	resetTokenLen             = 32
	verificationTokenLen      = 32
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
	ErrSessionExpired           = errors.New("会话已过期")
	ErrSessionNotFound          = errors.New("会话不存在")
	ErrResetTokenInvalid        = errors.New("重置链接无效或已过期")
	ErrResetTokenUsed           = errors.New("重置链接已被使用")
	ErrVerificationTokenInvalid = errors.New("验证链接无效或已过期")
	ErrVerificationTokenUsed    = errors.New("验证链接已被使用")
	ErrEmailAlreadyVerified     = errors.New("邮箱已验证")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

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

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Register(email, password string) (*model.User, *model.Session, error) {
	if !emailRegex.MatchString(email) {
		return nil, nil, ErrEmailInvalid
	}
	if len(password) < minPasswordLen {
		return nil, nil, ErrPasswordTooShort
	}
	if !validatePasswordComplexity(password) {
		return nil, nil, ErrPasswordTooWeak
	}

	var existing model.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, nil, err
	}

	user := &model.User{
		Email:        email,
		PasswordHash: string(hash),
	}
	if err := s.db.Create(user).Error; err != nil {
		return nil, nil, err
	}

	session, err := s.createSession(user.ID)
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
