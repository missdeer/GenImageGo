package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

const (
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"
	csrfTokenLen   = 32
)

type CSRFConfig struct {
	AllowedOrigin string
	Secure        bool
}

func CSRF(config CSRFConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				ensureCSRFCookie(w, r, config.Secure)
				next.ServeHTTP(w, r)
				return
			}

			if !validateOrigin(r, config.AllowedOrigin) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			cookieToken := getCSRFCookie(r)
			headerToken := r.Header.Get(CSRFHeaderName)

			if cookieToken == "" || headerToken == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func validateOrigin(r *http.Request, allowedOrigin string) bool {
	if allowedOrigin == "" {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		referer := r.Header.Get("Referer")
		if referer != "" {
			if u, err := url.Parse(referer); err == nil {
				origin = u.Scheme + "://" + u.Host
			}
		}
	}

	if origin == "" {
		return true
	}

	allowedOrigin = strings.TrimSuffix(allowedOrigin, "/")
	origin = strings.TrimSuffix(origin, "/")

	return strings.EqualFold(origin, allowedOrigin)
}

func getCSRFCookie(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, secure bool) {
	if _, err := r.Cookie(CSRFCookieName); err == nil {
		return
	}

	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate CSRF token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func RotateCSRFToken(w http.ResponseWriter, secure bool) string {
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
	return token
}
