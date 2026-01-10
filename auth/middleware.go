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
	"/login.html",
	"/register.html",
	"/forgot-password.html",
	"/reset-password.html",
	"/api/auth/login",
	"/api/auth/register",
	"/api/auth/forgot-password",
	"/api/auth/validate-reset-token",
	"/api/auth/reset-password",
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

func Middleware(authService *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				handleUnauthorized(w, r)
				return
			}

			user, err := authService.ValidateSession(cookie.Value)
			if err != nil {
				clearSessionCookie(w)
				handleUnauthorized(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "未授权访问", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login.html", http.StatusSeeOther)
}

func GetUserFromContext(ctx context.Context) *model.User {
	user, ok := ctx.Value(UserContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}

func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
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
