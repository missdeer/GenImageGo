package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Server struct {
	addr       string
	staticDir  string
	config     ServerConfig
	httpServer *http.Server
	httpClient *http.Client
}

type ServerConfig struct {
	Model        string
	BaseURL      string
	APIKey       string
	ModelSource  string
	BaseURLSource string
	APIKeySource string
}

func NewServer(addr, staticDir string, config ServerConfig) *Server {
	return &Server{
		addr:      addr,
		staticDir: staticDir,
		config:    config,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	if _, err := os.Stat(s.staticDir); os.IsNotExist(err) {
		return fmt.Errorf("静态资源目录不存在: %s", s.staticDir)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(s.staticDir))
	mux.HandleFunc("/generate-image", s.handleGenerateImage)
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
	fmt.Printf("静态资源目录: %s\n", s.staticDir)
	fmt.Println("按 Ctrl+C 停止服务器")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器启动失败: %w", err)
	}

	return nil
}

func (s *Server) handleGenerateImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("generate-image request from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}
	log.Printf("generate-image payload: %s", string(body))
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	modelSource := s.config.ModelSource
	model := s.config.Model
	if model == "" {
		modelSource = "default"
		model = Defaults.Model
	}
	apiKeySource := s.config.APIKeySource
	apiKey := s.config.APIKey
	if apiKey == "" {
		apiKeySource = "default"
		apiKey = Defaults.APIKey
	}
	if apiKey == "" {
		log.Printf("generate-image missing API key (source=%s)", apiKeySource)
		http.Error(w, "missing API key", http.StatusBadRequest)
		return
	}
	baseURLSource := s.config.BaseURLSource
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURLSource = "default"
		baseURL = Defaults.BaseURL
	}
	if baseURL == "" {
		baseURLSource = "builtin"
		baseURL = DefaultGeminiBaseURL
	}
	log.Printf("generate-image upstream model=%s (source=%s) baseURL=%s (source=%s) apiKeySource=%s keyLen=%d", model, modelSource, baseURL, baseURLSource, apiKeySource, len(apiKey))
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("generate-image upstream error: %v", err)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("generate-image upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
