package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"genimage/model"

	"gorm.io/gorm"
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

	ImageGenerationPoints int
	EnhancePromptPoints   int
}

type Handler struct {
	config     Config
	httpClient *http.Client
	db         *gorm.DB
}

func New(config Config, db *gorm.DB) *Handler {
	return &Handler{
		config: config,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
		db: db,
	}
}

func (h *Handler) EnhancePrompt(w http.ResponseWriter, r *http.Request) {
	handleEnhancePrompt(h, w, r)
}

func (h *Handler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	handleGenerateImage(h, w, r)
}

type jsonErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := jsonErrorResponse{}
	resp.Error.Message = message
	_ = json.NewEncoder(w).Encode(resp)
}

func refundPoints(db *gorm.DB, userID uint, points int, operation string) {
	if db == nil || points <= 0 {
		return
	}
	if err := db.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("points", gorm.Expr("points + ?", points)).Error; err != nil {
		log.Printf("%s refund points failed: user=%d points=%d err=%v", operation, userID, points, err)
	} else {
		log.Printf("%s refund points: user=%d points=%d", operation, userID, points)
	}
}
