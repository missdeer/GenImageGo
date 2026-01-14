package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"genimage/auth"
	"genimage/model"
	"genimage/siteconfig"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.SiteConfig{}); err != nil {
		t.Fatalf("Failed to migrate test db: %v", err)
	}
	return db
}

func setupTestHandler(t *testing.T, db *gorm.DB) *SiteConfigHandler {
	configService := siteconfig.NewService(db)
	if err := configService.Load(); err != nil {
		t.Fatalf("Failed to load config service: %v", err)
	}
	authService := auth.NewService(db, configService)
	return NewSiteConfigHandler(authService, configService)
}

func createTestUser(t *testing.T, db *gorm.DB, userType model.UserType, email string) *model.User {
	user := &model.User{
		Email:        email,
		Type:         userType,
		PasswordHash: "hash",
		ReferralCode: email, // Use email as unique referral code for tests
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user
}

func TestSiteConfigHandler_GetConfig(t *testing.T) {
	db := setupTestDB(t)
	handler := setupTestHandler(t, db)

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/site-config", nil)
		w := httptest.NewRecorder()
		handler.Handle(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status Unauthorized, got %v", w.Code)
		}
	})

	t.Run("Forbidden", func(t *testing.T) {
		user := createTestUser(t, db, model.UserTypeNormal, "normal@example.com")
		req := httptest.NewRequest(http.MethodGet, "/api/admin/site-config", nil)
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status Forbidden, got %v", w.Code)
		}
	})

	t.Run("Success", func(t *testing.T) {
		user := createTestUser(t, db, model.UserTypeSuperAdmin, "admin@example.com")
		req := httptest.NewRequest(http.MethodGet, "/api/admin/site-config", nil)
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		configs, ok := resp["configs"].([]interface{})
		if !ok {
			t.Fatalf("Response 'configs' is not a list")
		}
		if len(configs) != 3 { // Should match DefaultSiteConfigs length
			t.Errorf("Expected 3 configs, got %d", len(configs))
		}
	})
}

func TestSiteConfigHandler_UpdateConfig(t *testing.T) {
	db := setupTestDB(t)
	handler := setupTestHandler(t, db)
	user := createTestUser(t, db, model.UserTypeSuperAdmin, "admin@example.com")

	t.Run("Success", func(t *testing.T) {
		payload := map[string]interface{}{
			"configs": []map[string]string{
				{"key": model.ConfigKeyDailyLoginPoints, "value": "50"},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/site-config", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("InvalidValue", func(t *testing.T) {
		payload := map[string]interface{}{
			"configs": []map[string]string{
				{"key": model.ConfigKeyDailyLoginPoints, "value": "abc"},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/site-config", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", w.Code)
		}
	})

	t.Run("UnknownKey", func(t *testing.T) {
		payload := map[string]interface{}{
			"configs": []map[string]string{
				{"key": "unknown_key", "value": "100"},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/site-config", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", w.Code)
		}
	})

	t.Run("OutOfRange", func(t *testing.T) {
		payload := map[string]interface{}{
			"configs": []map[string]string{
				{"key": model.ConfigKeyDailyLoginPoints, "value": "2000"},
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/admin/site-config", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.UserContextKey, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler.Handle(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", w.Code)
		}
	})
}
