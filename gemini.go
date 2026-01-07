package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com"
	DefaultLocation      = "us-central1"
)

// GeminiClient Gemini API 客户端
type GeminiClient struct {
	config     GeminiConfig
	httpClient *http.Client
}

// NewGeminiClient 创建新的 Gemini 客户端
func NewGeminiClient(ctx context.Context, config GeminiConfig) (*GeminiClient, error) {
	if config.Vertex {
		// Vertex AI 模式需要凭证
		if config.Credentials != "" {
			if !FileExists(config.Credentials) {
				return nil, fmt.Errorf("credentials file not found: %s", config.Credentials)
			}
		}
	} else {
		// API Key 模式
		if config.APIKey == "" {
			return nil, fmt.Errorf("API key is required for non-Vertex AI mode")
		}
	}

	return &GeminiClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}, nil
}

// Close 关闭客户端
func (c *GeminiClient) Close() error {
	return nil
}

// GeminiRequest Gemini API 请求结构
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent 内容结构
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart 内容部分
type GeminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inline_data,omitempty"`
}

// GeminiInlineData 内联数据（图片）
type GeminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

// GeminiGenerationConfig 生成配置
type GeminiGenerationConfig struct {
	ResponseModalities []string           `json:"response_modalities,omitempty"`
	ResponseMIMEType   string             `json:"response_mime_type,omitempty"`
	ImageConfig        *GeminiImageConfig `json:"image_config,omitempty"`
}

// GeminiImageConfig 图片生成配置
type GeminiImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

// GeminiResponse Gemini API 响应结构
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	Error      *GeminiError      `json:"error,omitempty"`
}

// GeminiCandidate 候选结果
type GeminiCandidate struct {
	Content      *GeminiResponseContent `json:"content"`
	FinishReason string                 `json:"finishReason"`
}

// GeminiResponseContent 响应内容
type GeminiResponseContent struct {
	Parts []GeminiResponsePart `json:"parts"`
	Role  string               `json:"role"`
}

// GeminiResponsePart 响应部分
type GeminiResponsePart struct {
	Text       string                    `json:"text,omitempty"`
	InlineData *GeminiResponseInlineData `json:"inlineData,omitempty"`
}

// GeminiResponseInlineData 响应中的内联数据
type GeminiResponseInlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiError API 错误
type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// GenerateImageViaChat 使用 Gemini API 生成图片
func (c *GeminiClient) GenerateImageViaChat(ctx context.Context, model, prompt string, referenceImages []string, aspectRatio, resolution string) ([]byte, string, error) {
	// 构建请求内容
	var parts []GeminiPart

	// 添加文本提示
	parts = append(parts, GeminiPart{Text: prompt})

	// 添加参考图片
	for _, imagePath := range referenceImages {
		if !FileExists(imagePath) {
			fmt.Printf("警告: 参考图片不存在: %s\n", imagePath)
			continue
		}

		imgData, err := os.ReadFile(imagePath)
		if err != nil {
			fmt.Printf("警告: 无法读取参考图片 %s: %v\n", imagePath, err)
			continue
		}

		mimeType := GetMIMEType(imagePath)
		b64Data := base64.StdEncoding.EncodeToString(imgData)

		parts = append(parts, GeminiPart{
			InlineData: &GeminiInlineData{
				MIMEType: mimeType,
				Data:     b64Data,
			},
		})
	}

	// 构建生成配置
	genConfig := &GeminiGenerationConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	// 添加图片配置
	if aspectRatio != "" || resolution != "" {
		imageConfig := &GeminiImageConfig{}
		if aspectRatio != "" {
			imageConfig.AspectRatio = aspectRatio
		}
		if resolution != "" {
			imageConfig.ImageSize = resolution
		}
		genConfig.ImageConfig = imageConfig
	}

	// 构建请求
	request := GeminiRequest{
		Contents: []GeminiContent{
			{Parts: parts},
		},
		GenerationConfig: genConfig,
	}

	// 构建 URL
	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = DefaultGeminiBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// API URL 格式: {baseURL}/v1beta/models/{model}:generateContent?key={apiKey}
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", baseURL, model, c.config.APIKey)

	// 序列化请求
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, "", fmt.Errorf("解析响应失败: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, "", fmt.Errorf("API 错误: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, "", fmt.Errorf("API 返回空候选列表")
	}

	// 解析响应内容
	var resultImage []byte
	var resultTexts []string

	for _, candidate := range geminiResp.Candidates {
		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				resultTexts = append(resultTexts, part.Text)
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				// 解码 base64 图片
				imgBytes, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					fmt.Printf("警告: 解码图片失败: %v\n", err)
					continue
				}
				resultImage = imgBytes
			}
		}
	}

	var textResponse string
	if len(resultTexts) > 0 {
		textResponse = strings.Join(resultTexts, "\n")
	}

	return resultImage, textResponse, nil
}
