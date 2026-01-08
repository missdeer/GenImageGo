package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	enhancePrompt string
)

type Server struct {
	addr       string
	staticDir  string
	config     ServerConfig
	httpServer *http.Server
	httpClient *http.Client
}

type ServerConfig struct {
	Model         string
	BaseURL       string
	APIKey        string
	ModelSource   string
	BaseURLSource string
	APIKeySource  string
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
	staticSubFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("无法访问嵌入的静态资源: %w", err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(staticSubFS))
	mux.HandleFunc("/generate-image", s.handleGenerateImage)
	mux.HandleFunc("/enhance-prompt", s.handleEnhancePrompt)
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

type EnhancePromptRequest struct {
	Prompt string `json:"prompt"`
}

func (s *Server) handleEnhancePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	log.Printf("enhance-prompt request from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	reqPrompt := string(body)

	systemPrompt := enhancePrompt
	if systemPrompt == "" {
		systemPrompt = embeddedEnhancePrompt
	}
	if systemPrompt == "" {
		log.Printf("enhance-prompt system prompt not configured")
		http.Error(w, "system prompt not configured", http.StatusInternalServerError)
		return
	}

	apiKey := s.config.APIKey
	if apiKey == "" {
		apiKey = Defaults.APIKey
	}
	if apiKey == "" {
		log.Printf("enhance-prompt missing API key")
		http.Error(w, "missing API key", http.StatusBadRequest)
		return
	}
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = Defaults.BaseURL
	}
	if baseURL == "" {
		baseURL = DefaultGeminiBaseURL
	}

	model := "gemini-3-pro-preview"
	geminiReq := GeminiRequest{
		SystemInstruction: &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemPrompt}},
		},
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: reqPrompt}},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			ResponseMIMETypeCamel: "application/json",
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		log.Printf("enhance-prompt failed to marshal request: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimSuffix(baseURL, "/"),
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(upstreamReq)
	if err != nil {
		log.Printf("enhance-prompt upstream error: %v", err)
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	log.Printf("enhance-prompt upstream status=%d duration=%s", resp.StatusCode, time.Since(start))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("enhance-prompt failed to read response: %v", err)
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("enhance-prompt upstream error response: %s", string(respBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		log.Printf("enhance-prompt failed to parse response: %v", err)
		http.Error(w, "failed to parse upstream response", http.StatusBadGateway)
		return
	}

	if geminiResp.Error != nil {
		log.Printf("enhance-prompt API error: %s", geminiResp.Error.Message)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write(respBody)
		return
	}

	if len(geminiResp.Candidates) == 0 || geminiResp.Candidates[0].Content == nil ||
		len(geminiResp.Candidates[0].Content.Parts) == 0 {
		log.Printf("enhance-prompt empty response from API")
		http.Error(w, "empty response from API", http.StatusBadGateway)
		return
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
	log.Printf("enhance-prompt result text: %s", text)
	if strings.HasPrefix("```json", text) && strings.HasSuffix("```", text) {
		text = text[len("```json"):]
		text = text[:len(text)-len("```")]
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
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
