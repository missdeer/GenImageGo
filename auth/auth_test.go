package auth

import (
	"os"
	"testing"

	"genimage/model"
)

func setupTestDB(t *testing.T) *model.User {
	t.Helper()
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	svc := NewService(db)
	user, _, err := svc.Register("test@example.com", "Password123")
	if err != nil {
		t.Fatalf("failed to register test user: %v", err)
	}
	return user
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestRegister(t *testing.T) {
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	svc := NewService(db)

	tests := []struct {
		name    string
		email   string
		password string
		wantErr error
	}{
		{"valid", "valid@example.com", "Password123", nil},
		{"invalid email format", "notanemail", "Password123", ErrEmailInvalid},
		{"password too short", "user2@example.com", "Ab1", ErrPasswordTooShort},
		{"password no uppercase", "user3@example.com", "password123", ErrPasswordTooWeak},
		{"password no lowercase", "user4@example.com", "PASSWORD123", ErrPasswordTooWeak},
		{"password no digit", "user5@example.com", "PasswordABC", ErrPasswordTooWeak},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Register(tt.email, tt.password)
			if err != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterDuplicate(t *testing.T) {
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	svc := NewService(db)

	_, _, err = svc.Register("test@example.com", "Password123")
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, _, err = svc.Register("test@example.com", "Password456")
	if err != ErrEmailExists {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	svc := NewService(db)

	_, _, err = svc.Register("test@example.com", "Password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	tests := []struct {
		name    string
		email   string
		password string
		wantErr error
	}{
		{"valid login", "test@example.com", "Password123", nil},
		{"wrong password", "test@example.com", "WrongPass123", ErrInvalidCredentials},
		{"unknown user", "unknown@example.com", "Password123", ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Login(tt.email, tt.password)
			if err != tt.wantErr {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSessionValidation(t *testing.T) {
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	svc := NewService(db)

	user, session, err := svc.Register("test@example.com", "Password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	validatedUser, err := svc.ValidateSession(session.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if validatedUser.ID != user.ID {
		t.Errorf("ValidateSession returned wrong user: got %d, want %d", validatedUser.ID, user.ID)
	}

	_, err = svc.ValidateSession("invalidtoken")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestLogout(t *testing.T) {
	db, err := model.InitDB(":memory:")
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	svc := NewService(db)

	_, session, err := svc.Register("test@example.com", "Password123")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	err = svc.Logout(session.Token)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	_, err = svc.ValidateSession(session.Token)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after logout, got %v", err)
	}
}
