package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFMiddleware_SafeMethods(t *testing.T) {
	handler := CSRF(CSRFConfig{Secure: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		req := httptest.NewRequest(method, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s request should pass, got %d", method, rec.Code)
		}

		cookies := rec.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == CSRFCookieName {
				found = true
				if len(c.Value) != 64 {
					t.Errorf("CSRF token should be 64 chars, got %d", len(c.Value))
				}
			}
		}
		if !found {
			t.Errorf("%s request should set CSRF cookie", method)
		}
	}
}

func TestCSRFMiddleware_PostWithoutToken(t *testing.T) {
	handler := CSRF(CSRFConfig{Secure: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF token should be forbidden, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_PostWithValidToken(t *testing.T) {
	handler := CSRF(CSRFConfig{Secure: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()

	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST with valid CSRF token should pass, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_PostWithMismatchedToken(t *testing.T) {
	handler := CSRF(CSRFConfig{Secure: false})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "token1"})
	req.Header.Set(CSRFHeaderName, "token2")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST with mismatched CSRF token should be forbidden, got %d", rec.Code)
	}
}

func TestCSRFMiddleware_OriginValidation(t *testing.T) {
	handler := CSRF(CSRFConfig{
		AllowedOrigin: "https://example.com",
		Secure:        false,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()

	// Valid origin
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST with valid origin should pass, got %d", rec.Code)
	}

	// Invalid origin
	req = httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)
	req.Header.Set("Origin", "https://evil.com")

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST with invalid origin should be forbidden, got %d", rec.Code)
	}
}

func TestResponseRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w)

	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusCreated)
	rec.Write([]byte(`{"id": 1}`))

	if rec.Code() != http.StatusCreated {
		t.Errorf("Code should be 201, got %d", rec.Code())
	}

	if !strings.Contains(string(rec.Body()), `{"id": 1}`) {
		t.Errorf("Body should contain JSON, got %s", string(rec.Body()))
	}

	if rec.Size() != 9 {
		t.Errorf("Size should be 9, got %d", rec.Size())
	}
}

func TestResponseRecorder_MaxSize(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w)
	rec.maxSize = 10

	data := []byte("12345678901234567890")
	rec.Write(data)

	if rec.Size() != 10 {
		t.Errorf("Size should be capped at 10, got %d", rec.Size())
	}

	if len(rec.Body()) != 10 {
		t.Errorf("Body should be capped at 10 bytes, got %d", len(rec.Body()))
	}
}
