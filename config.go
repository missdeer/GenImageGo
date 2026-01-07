package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// APIService 表示 API 服务类型
type APIService string

const (
	APIServiceOpenAI   APIService = "openai"
	APIServiceGemini   APIService = "gemini"
	APIServiceVertexAI APIService = "vertexai"
)

// ValidAPIServices 返回所有有效的 API 服务类型
func ValidAPIServices() []APIService {
	return []APIService{APIServiceOpenAI, APIServiceGemini, APIServiceVertexAI}
}

// IsValid 检查 API 服务类型是否有效
func (s APIService) IsValid() bool {
	switch s {
	case APIServiceOpenAI, APIServiceGemini, APIServiceVertexAI:
		return true
	}
	return false
}

// Defaults 默认配置值
var Defaults = struct {
	APIService  APIService
	Model       string
	BaseURL     string
	APIKey      string
	Output      string
	Location    string
	AspectRatio string
	Resolution  string
	Prompt      string
}{
	APIService:  APIServiceGemini,
	Model:       "gemini-3-pro-image-preview",
	BaseURL:     "http://192.168.233.166:8317",
	APIKey:      "your-api-key-1",
	Output:      "output.jpg",
	Location:    "us-central1",
	AspectRatio: "3:4",
	Resolution:  "4K",
	Prompt:      "美丽的中国少女穿着浅色碎花短裙在小溪里玩水，头戴遮阳帽，小溪旁巨大的树荫挡住了阳光",
}

// Config 表示从 JSON 文件加载的配置
type Config struct {
	APIService  string   `json:"api_service,omitempty"`
	Model       string   `json:"model,omitempty"`
	BaseURL     string   `json:"base_url,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
	Output      string   `json:"output,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	PromptFile  string   `json:"prompt_file,omitempty"`
	Project     string   `json:"project,omitempty"`
	Location    string   `json:"location,omitempty"`
	Credentials string   `json:"credentials,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	Images      []string `json:"images,omitempty"`
}

// LoadConfig 从 JSON 文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在: %s", configPath)
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("配置文件 JSON 格式错误: %w", err)
	}

	return &config, nil
}

// OpenAIConfig 表示 OpenAI 兼容 API 的配置
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// GeminiConfig 表示 Gemini API 的配置
type GeminiConfig struct {
	APIKey      string
	BaseURL     string
	Vertex      bool
	Project     string
	Location    string
	Credentials string
}

// GetConfigValue 获取配置值，优先级：命令行参数 > 配置文件 > 默认值
func GetConfigValue(cliValue, configValue, defaultValue string) string {
	if cliValue != "" {
		return cliValue
	}
	if configValue != "" {
		return configValue
	}
	return defaultValue
}

// GetConfigValuePtr 类似 GetConfigValue，但处理指针类型
func GetConfigValuePtr(cliValue *string, configValue, defaultValue string) string {
	if cliValue != nil && *cliValue != "" {
		return *cliValue
	}
	if configValue != "" {
		return configValue
	}
	return defaultValue
}
