package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"genimage/handler"
)

type Server struct {
	addr       string
	staticDir  string
	config     ServerConfig
	httpServer *http.Server
	handler    *handler.Handler
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

func NewServer(addr, staticDir string, config ServerConfig) *Server {
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

	return &Server{
		addr:      addr,
		staticDir: staticDir,
		config:    config,
		handler:   h,
	}
}

func (s *Server) Start() error {
	staticSubFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("无法访问嵌入的静态资源: %w", err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(staticSubFS))
	mux.HandleFunc("/generate-image", s.handler.GenerateImage)
	mux.HandleFunc("/enhance-prompt", s.handler.EnhancePrompt)
	mux.Handle("/", fileServer)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
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
