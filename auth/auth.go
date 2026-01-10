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
	bcryptCost      = 10
	sessionTokenLen = 32
	sessionDuration = 7 * 24 * time.Hour
	minPasswordLen  = 6
)

var (
	ErrEmailInvalid       = errors.New("邮箱格式不正确")
	ErrEmailExists        = errors.New("该邮箱已被注册")
	ErrPasswordTooShort   = errors.New("密码长度至少为 6 个字符")
	ErrPasswordTooWeak    = errors.New("密码必须包含大写字母、小写字母和数字")
	ErrInvalidCredentials = errors.New("邮箱或密码错误")
	ErrSessionExpired     = errors.New("会话已过期")
	ErrSessionNotFound    = errors.New("会话不存在")
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
