package auth

import (
	"context"
	"net/http"
	"strings"

	"genimage/model"
)

type contextKey string

const UserContextKey contextKey = "user"

const SessionCookieName = "session_token"

var publicPaths = []string{
	"/login",
	"/register",
	"/forgot-password",
	"/reset-password",
	"/api/auth/login",
	"/api/auth/register",
	"/api/auth/forgot-password",
	"/api/auth/validate-reset-token",
	"/api/auth/reset-password",
	"/api/auth/verify-email",
	"/css/",
	"/js/",
	"/components/",
}

var authPagePaths = map[string]bool{
	"/login":           true,
	"/register":        true,
	"/forgot-password": true,
	"/reset-password":  true,
}

var unverifiedAllowedPaths = []string{
	"/verify-pending",
	"/api/auth/me",
	"/api/auth/logout",
	"/api/auth/resend-verification",
	"/api/auth/verify-email",
	"/css/",
	"/js/",
	"/components/",
}

func isPublicPath(path string) bool {
	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isAuthPage(path string) bool {
	return authPagePaths[path]
}

func isUnverifiedAllowedPath(path string) bool {
	for _, p := range unverifiedAllowedPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func Middleware(authService *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err == nil {
				if user, err := authService.ValidateSession(cookie.Value); err == nil {
					if isAuthPage(r.URL.Path) {
						http.Redirect(w, r, "/", http.StatusSeeOther)
						return
					}

					if !user.EmailVerified && !isUnverifiedAllowedPath(r.URL.Path) {
						handleUnverified(w, r)
						return
					}

					ctx := context.WithValue(r.Context(), UserContextKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				clearSessionCookie(w)
			}

			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			handleUnauthorized(w, r)
		})
	}
}

func handleUnverified(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"请先验证邮箱"}`))
		return
	}
	http.Redirect(w, r, "/verify-pending", http.StatusSeeOther)
}

func handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "未授权访问", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func GetUserFromContext(ctx context.Context) *model.User {
	user, ok := ctx.Value(UserContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}

func SetSessionCookie(w http.ResponseWriter, token string) {
	SetSessionCookieWithExpiry(w, token, true)
}

func SetSessionCookieWithExpiry(w http.ResponseWriter, token string, rememberMe bool) {
	maxAge := 0
	if rememberMe {
		maxAge = 7 * 24 * 60 * 60
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
