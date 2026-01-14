package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"genimage/auth"
	"genimage/handler"
	"genimage/mail"
	"genimage/middleware"

	"gorm.io/gorm"
)

type Server struct {
	addr           string
	staticDir      string
	config         ServerConfig
	httpServer     *http.Server
	handler        *handler.Handler
	authHandler    *handler.AuthHandler
	adminHandler   *handler.AdminHandler
	pointsHandler  *handler.PointsHandler
	authService    *auth.Service
	db             *gorm.DB
	stopChan       chan struct{}
}

type ServerConfig struct {
	APIService            string
	Model                 string
	TextModel             string
	BaseURL               string
	APIKey                string
	ModelSource           string
	BaseURLSource         string
	APIKeySource          string
	SMTP                  *SMTPConfig
	BaseWebURL            string
	DailyLoginPoints      int
	ImageGenerationPoints int
	EnhancePromptPoints   int
	SecureCookies         bool // Set to true for HTTPS production
}

func NewServer(addr, staticDir string, config ServerConfig, db *gorm.DB) *Server {
	enhancePromptText := embeddedEnhancePrompt

	imageGenerationPoints := config.ImageGenerationPoints
	if imageGenerationPoints == 0 {
		imageGenerationPoints = 20
	}
	enhancePromptPoints := config.EnhancePromptPoints
	if enhancePromptPoints == 0 {
		enhancePromptPoints = 4
	}

	h := handler.New(handler.Config{
		APIService:    config.APIService,
		Model:         config.Model,
		TextModel:     config.TextModel,
		BaseURL:       config.BaseURL,
		APIKey:        config.APIKey,
		ModelSource:   config.ModelSource,
		BaseURLSource: config.BaseURLSource,
		APIKeySource:  config.APIKeySource,

		DefaultAPIService: string(Defaults.APIService),
		DefaultModel:      Defaults.Model,
		DefaultTextModel:  Defaults.TextModel,
		DefaultBaseURL:    Defaults.BaseURL,
		DefaultAPIKey:     Defaults.APIKey,
		EnhancePromptText: enhancePromptText,

		ImageGenerationPoints: imageGenerationPoints,
		EnhancePromptPoints:   enhancePromptPoints,
	}, db)

	authService := auth.NewService(db, config.DailyLoginPoints)

	var mailService *mail.Service
	if config.SMTP != nil && config.SMTP.Host != "" {
		mailService = mail.NewService(mail.SMTPConfig{
			Host:     config.SMTP.Host,
			Port:     config.SMTP.Port,
			Username: config.SMTP.Username,
			Password: config.SMTP.Password,
			From:     config.SMTP.From,
		})
	}

	baseWebURL := config.BaseWebURL
	if baseWebURL == "" {
		baseWebURL = fmt.Sprintf("http://%s", addr)
	}

	authHandler := handler.NewAuthHandler(authService, mailService, baseWebURL)
	adminHandler := handler.NewAdminHandler(authService)
	pointsHandler := handler.NewPointsHandler(authService)

	return &Server{
		addr:           addr,
		staticDir:      staticDir,
		config:         config,
		handler:        h,
		authHandler:    authHandler,
		adminHandler:   adminHandler,
		pointsHandler:  pointsHandler,
		authService:    authService,
		db:             db,
		stopChan:       make(chan struct{}),
	}
}

func (s *Server) Start() error {
	staticSubFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("无法访问嵌入的静态资源: %w", err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(staticSubFS))

	mux.HandleFunc("/api/auth/register", s.authHandler.Register)
	mux.HandleFunc("/api/auth/login", s.authHandler.Login)
	mux.HandleFunc("/api/auth/logout", s.authHandler.Logout)
	mux.HandleFunc("/api/auth/me", s.authHandler.Me)
	mux.HandleFunc("/api/auth/profile", s.authHandler.Profile)
	mux.HandleFunc("/api/auth/change-password", s.authHandler.ChangePassword)
	mux.HandleFunc("/api/auth/forgot-password", s.authHandler.ForgotPassword)
	mux.HandleFunc("/api/auth/validate-reset-token", s.authHandler.ValidateResetToken)
	mux.HandleFunc("/api/auth/reset-password", s.authHandler.ResetPassword)
	mux.HandleFunc("/api/auth/resend-verification", s.authHandler.ResendVerification)
	mux.HandleFunc("/api/auth/verify-email", s.authHandler.VerifyEmail)

	mux.HandleFunc("/generate-image", s.handler.GenerateImage)
	mux.HandleFunc("/enhance-prompt", s.handler.EnhancePrompt)

	mux.HandleFunc("/api/admin/users", s.adminHandler.ListUsers)
	mux.HandleFunc("/api/admin/organizations", s.adminHandler.ListOrganizations)
	mux.HandleFunc("/api/admin/users/toggle-disabled", s.adminHandler.ToggleUserDisabled)
	mux.HandleFunc("/api/admin/users/allocate-points", s.adminHandler.AllocateUserPoints)
	mux.HandleFunc("/api/admin/users/delete", s.adminHandler.DeleteUser)
	mux.HandleFunc("/api/admin/users/update-memberships", s.adminHandler.UpdateUserMemberships)

	mux.HandleFunc("/api/admin/orgs", s.adminHandler.ListOrgsForManagement)
	mux.HandleFunc("/api/admin/orgs/create", s.adminHandler.CreateOrganization)
	mux.HandleFunc("/api/admin/orgs/delete", s.adminHandler.DeleteOrganization)
	mux.HandleFunc("/api/admin/orgs/update-points", s.adminHandler.UpdateOrganizationPoints)
	mux.HandleFunc("/api/admin/orgs/update-name", s.adminHandler.UpdateOrganizationName)

	mux.HandleFunc("/api/user/points/history", s.pointsHandler.UserPointsHistory)
	mux.HandleFunc("/api/admin/user/points/history", s.pointsHandler.AdminUserPointsHistory)
	mux.HandleFunc("/api/admin/org/points/history", s.pointsHandler.AdminOrgPointsHistory)

	serveHTML := func(filename string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = "/" + filename
			fileServer.ServeHTTP(w, r)
		}
	}
	mux.HandleFunc("/login", serveHTML("login.html"))
	mux.HandleFunc("/register", serveHTML("register.html"))
	mux.HandleFunc("/forgot-password", serveHTML("forgot-password.html"))
	mux.HandleFunc("/reset-password", serveHTML("reset-password.html"))
	mux.HandleFunc("/verify-pending", serveHTML("verify-pending.html"))
	mux.HandleFunc("/profile", serveHTML("profile.html"))
	mux.HandleFunc("/admin/users", serveHTML("admin/users.html"))
	mux.HandleFunc("/admin/organizations", serveHTML("admin/organizations.html"))

	htmlRedirects := map[string]string{
		"/login.html":           "/login",
		"/register.html":        "/register",
		"/forgot-password.html": "/forgot-password",
		"/reset-password.html":  "/reset-password",
		"/verify-pending.html":  "/verify-pending",
		"/profile.html":         "/profile",
		"/index.html":           "/",
	}

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") {
			if redirect, ok := htmlRedirects[r.URL.Path]; ok {
				http.Redirect(w, r, redirect, http.StatusMovedPermanently)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}))

	authMiddleware := auth.Middleware(s.authService)

	csrfMiddleware := middleware.CSRF(middleware.CSRFConfig{
		AllowedOrigin: s.config.BaseWebURL,
		Secure:        s.config.SecureCookies,
	})

	idempotencyMiddleware := middleware.Idempotency(s.db, middleware.IdempotencyConfig{
		EnabledPaths: []string{"/generate-image", "/enhance-prompt"},
	})

	// Chain: Auth -> CSRF -> Idempotency -> Handler
	wrappedHandler := authMiddleware(csrfMiddleware(idempotencyMiddleware(mux)))

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: wrappedHandler,
	}

	// Start background cleanup for expired idempotency keys
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := middleware.CleanExpiredIdempotencyKeys(s.db); err != nil {
					fmt.Printf("清理过期幂等键失败: %v\n", err)
				}
			case <-s.stopChan:
				return
			}
		}
	}()

	// Start background cleanup for expired sessions
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.authService.CleanExpiredSessions(); err != nil {
					fmt.Printf("清理过期会话失败: %v\n", err)
				}
			case <-s.stopChan:
				return
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n正在关闭服务器...")
		close(s.stopChan)
		s.httpServer.Close()
	}()

	fmt.Printf("服务器启动成功: http://%s\n", s.addr)
	fmt.Println("静态资源: embedded")
	fmt.Println("按 Ctrl+C 停止服务器")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器启动失败: %w", err)
	}

	return nil
}
