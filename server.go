package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"genimage/auth"
	"genimage/handler"

	"gorm.io/gorm"
)

type Server struct {
	addr        string
	staticDir   string
	config      ServerConfig
	httpServer  *http.Server
	handler     *handler.Handler
	authHandler *handler.AuthHandler
	authService *auth.Service
	db          *gorm.DB
}

type ServerConfig struct {
	APIService    string
	Model         string
	TextModel     string
	BaseURL       string
	APIKey        string
	ModelSource   string
	BaseURLSource string
	APIKeySource  string
}

func NewServer(addr, staticDir string, config ServerConfig, db *gorm.DB) *Server {
	enhancePromptText := embeddedEnhancePrompt

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
	})

	authService := auth.NewService(db)
	authHandler := handler.NewAuthHandler(authService)

	return &Server{
		addr:        addr,
		staticDir:   staticDir,
		config:      config,
		handler:     h,
		authHandler: authHandler,
		authService: authService,
		db:          db,
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

	mux.HandleFunc("/generate-image", s.handler.GenerateImage)
	mux.HandleFunc("/enhance-prompt", s.handler.EnhancePrompt)
	mux.Handle("/", fileServer)

	authMiddleware := auth.Middleware(s.authService)
	wrappedHandler := authMiddleware(mux)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: wrappedHandler,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n正在关闭服务器...")
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
