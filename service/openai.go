package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"genimage/util"
)

const DefaultOpenAIBaseURL = "https://api.openai.com/v1"

// OpenAIConfig 表示 OpenAI 兼容 API 的配置
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// OpenAIClient OpenAI 兼容 API 客户端
type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// NewOpenAIClient 创建新的 OpenAI 客户端
func NewOpenAIClient(config OpenAIConfig) (*OpenAIClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for OpenAI-compatible mode")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	// 移除末尾的斜杠
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OpenAIClient{
		APIKey:  config.APIKey,
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 300 * time.Second,
		},
	}, nil
}

// ChatMessage 表示聊天消息
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// ContentPart 表示消息内容的一部分
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL 表示图片 URL
type ImageURL struct {
	URL string `json:"url"`
}

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// GenerateImageViaChat 使用 chat completions API 生成图片
// 返回图片字节和文本响应
func (c *OpenAIClient) GenerateImageViaChat(model, prompt string, referenceImages []string) ([]byte, string, error) {
	// 构建消息内容
	var content []ContentPart

	// 添加文本提示
	content = append(content, ContentPart{
		Type: "text",
		Text: prompt,
	})

	// 添加参考图片
	for _, imagePath := range referenceImages {
		b64, mimeType, err := util.EncodeImageForAPI(imagePath)
		if err != nil {
			fmt.Printf("警告: 无法读取参考图片 %s: %v\n", imagePath, err)
			continue
		}

		content = append(content, ContentPart{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL: fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
			},
		})
	}

	// 构建请求
	req := ChatCompletionRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	// 发送请求
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, "", fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, "", fmt.Errorf("解析响应失败: %w", err)
	}

	if chatResp.Error != nil {
		return nil, "", fmt.Errorf("API 错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, "", fmt.Errorf("API 返回空选择列表")
	}

	responseContent := chatResp.Choices[0].Message.Content
	if responseContent == "" {
		return nil, "", fmt.Errorf("API 返回的消息内容为空")
	}

	// 解析 markdown 格式的图片: ![image](data:image/xxx;base64,...)
	imageBytes, err := parseImageFromMarkdown(responseContent)
	if err != nil {
		// 如果没有找到图片，返回文本响应
		return nil, responseContent, nil
	}

	return imageBytes, "", nil
}

// parseImageFromMarkdown 从 markdown 格式解析 base64 图片
func parseImageFromMarkdown(content string) ([]byte, error) {
	formats := []string{"jpeg", "png", "gif", "webp"}

	for _, imgFmt := range formats {
		marker := fmt.Sprintf("![image](data:image/%s;base64,", imgFmt)
		if strings.Contains(content, marker) {
			parts := strings.Split(content, marker)
			if len(parts) >= 2 {
				// 提取 base64 数据（去掉结尾的括号）
				b64Data := strings.TrimSuffix(parts[1], ")")
				// 找到 base64 数据的结尾
				if idx := strings.Index(b64Data, ")"); idx != -1 {
					b64Data = b64Data[:idx]
				}
				return base64.StdEncoding.DecodeString(b64Data)
			}
		}
	}

	// 尝试使用正则表达式匹配
	re := regexp.MustCompile(`!\[image\]\(data:image/\w+;base64,([A-Za-z0-9+/=]+)\)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return base64.StdEncoding.DecodeString(matches[1])
	}

	return nil, fmt.Errorf("未找到图片数据")
}
