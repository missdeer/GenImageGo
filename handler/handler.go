package handler

import (
	"net/http"
	"time"
)

type Config struct {
	APIService    string
	Model         string
	TextModel     string
	BaseURL       string
	APIKey        string
	ModelSource   string
	BaseURLSource string
	APIKeySource  string

	DefaultAPIService string
	DefaultModel      string
	DefaultTextModel  string
	DefaultBaseURL    string
	DefaultAPIKey     string
	EnhancePromptText string
}

type Handler struct {
	config     Config
	httpClient *http.Client
}

func New(config Config) *Handler {
	return &Handler{
		config: config,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (h *Handler) EnhancePrompt(w http.ResponseWriter, r *http.Request) {
	handleEnhancePrompt(h, w, r)
}

func (h *Handler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	handleGenerateImage(h, w, r)
}
