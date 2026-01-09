package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"genimage/service"
	"genimage/util"

	"github.com/spf13/pflag"
)

// 可用的模型列表
var AvailableModels = []string{
	"gemini-3-pro-image-preview (CLIProxyAPI gemini/vertexai 模式)",
	"gemini-3-pro-image (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-3x4 (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-4x3 (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-9x16 (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-16x9 (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-4k (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-16x9-4k (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-9x16-4k (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-3x4-4k (AntiGravity-Manager openai 模式)",
	"gemini-3-pro-image-4x3-4k (AntiGravity-Manager openai 模式)",
}

// CLI 参数
var (
	configFile  string
	apiService  string
	model       string
	textModel   string
	prompt      string
	promptFile  string
	baseURL     string
	apiKey      string
	output      string
	project     string
	location    string
	credentials string
	aspectRatio string
	resolution  string
	showHelp    bool
	serve       bool
	serverAddr  string
	staticDir   string
)

func init() {
	// 配置文件
	pflag.StringVarP(&configFile, "config", "c", "", "从 JSON 配置文件读取配置（命令行参数优先级更高）")

	// API 服务
	pflag.StringVarP(&apiService, "api-service", "s", "", "API 服务类型（默认: gemini）\n可选值: openai, gemini, vertexai")

	// 模型
	pflag.StringVarP(&model, "model", "m", "", "模型名称（默认: gemini-3-pro-image-preview）")
	pflag.StringVar(&textModel, "text-model", "", "文本模型名称，用于提示词优化等（默认: gemini-3-flash-preview）")

	// 提示词（互斥组）
	pflag.StringVarP(&prompt, "prompt", "p", "", "图片生成的提示词")
	pflag.StringVarP(&promptFile, "prompt-file", "f", "", "从文本文件读取提示词（不能与 -p/--prompt 同时使用）")

	// API 配置
	pflag.StringVarP(&baseURL, "base-url", "u", "", "API 基础 URL（默认: http://192.168.233.166:8317）")
	pflag.StringVarP(&apiKey, "api-key", "k", "", "API 密钥")
	pflag.StringVarP(&output, "output", "o", "", "输出图片文件名（默认: output.jpg）")

	// Vertex AI 专用参数
	pflag.StringVarP(&project, "project", "j", "", "Google Cloud 项目 ID（vertexai 模式必需）")
	pflag.StringVarP(&location, "location", "l", "", "Vertex AI 区域（默认: us-central1）")
	pflag.StringVarP(&credentials, "credentials", "x", "", "Google Cloud 凭证文件路径")

	// Gemini 图片生成参数
	pflag.StringVarP(&aspectRatio, "aspect-ratio", "t", "", "图片宽高比（gemini/vertexai 模式，默认: 3:4）\n可选值: 1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9, 21:9")
	pflag.StringVarP(&resolution, "resolution", "r", "", "图片分辨率（gemini/vertexai 模式，默认: 4K）\n可选值: 1K, 2K, 4K")

	// 帮助
	pflag.BoolVarP(&showHelp, "help", "h", false, "显示帮助信息")

	// Web 服务器模式
	pflag.BoolVar(&serve, "serve", false, "启动 Web 服务器模式")
	pflag.StringVar(&serverAddr, "addr", "127.0.0.1:8080", "服务器监听地址")
	pflag.StringVar(&staticDir, "static", "static", "静态资源目录")

	// 自定义用法
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项] [图片文件...]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "调用 OpenAI/Gemini/Vertex AI API 生成图片")
		fmt.Fprintln(os.Stderr, "选项:")
		pflag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\n可用的模型:")
		for _, m := range AvailableModels {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
	}
}

func main() {
	pflag.Parse()

	if showHelp {
		pflag.Usage()
		os.Exit(0)
	}

	// 加载配置文件（如果指定）
	var config *Config
	if configFile != "" {
		var err error
		config, err = LoadConfig(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
	}

	// Web 服务器模式
	if serve {
		apiSvcValue := getConfigValue(apiService, getConfigString(config, "api_service"), string(Defaults.APIService))
		modelValue, modelSource := getConfigValueWithSource(model, getConfigString(config, "model"), Defaults.Model)
		textModelValue := getConfigValue(textModel, getConfigString(config, "text_model"), Defaults.TextModel)
		baseURLValue, baseURLSource := getConfigValueWithSource(baseURL, getConfigString(config, "base_url"), Defaults.BaseURL)
		apiKeyValue, apiKeySource := getConfigValueWithSource(apiKey, getConfigString(config, "api_key"), Defaults.APIKey)

		server := NewServer(serverAddr, staticDir, ServerConfig{
			APIService:    apiSvcValue,
			Model:         modelValue,
			TextModel:     textModelValue,
			BaseURL:       baseURLValue,
			APIKey:        apiKeyValue,
			ModelSource:   modelSource,
			BaseURLSource: baseURLSource,
			APIKeySource:  apiKeySource,
		})
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 验证 -p 和 -f 互斥
	if prompt != "" && promptFile != "" {
		fmt.Fprintln(os.Stderr, "错误: -p/--prompt 和 -f/--prompt-file 不能同时使用")
		os.Exit(1)
	}

	// 获取位置参数（图片文件列表）
	images := pflag.Args()

	// 解析 API 服务类型
	apiSvc := getConfigValue(apiService, getConfigString(config, "api_service"), string(Defaults.APIService))
	if !APIService(apiSvc).IsValid() {
		fmt.Fprintf(os.Stderr, "错误: 无效的 API 服务类型: %s\n", apiSvc)
		fmt.Fprintf(os.Stderr, "可选值: %s\n", strings.Join(getAPIServiceNames(), ", "))
		os.Exit(1)
	}

	// 解析配置值
	modelName := getConfigValue(model, getConfigString(config, "model"), Defaults.Model)
	baseURLValue := getConfigValue(baseURL, getConfigString(config, "base_url"), Defaults.BaseURL)
	apiKeyValue := getConfigValue(apiKey, getConfigString(config, "api_key"), Defaults.APIKey)
	outputValue := getConfigValue(output, getConfigString(config, "output"), Defaults.Output)
	aspectRatioValue := getConfigValue(aspectRatio, getConfigString(config, "aspect_ratio"), Defaults.AspectRatio)
	resolutionValue := getConfigValue(resolution, getConfigString(config, "resolution"), Defaults.Resolution)

	// Vertex AI 专用参数
	projectValue := getConfigValue(project, getConfigString(config, "project"), "")
	locationValue := getConfigValue(location, getConfigString(config, "location"), Defaults.Location)
	credentialsValue := getConfigValue(credentials, getConfigString(config, "credentials"), "")

	// 验证 Vertex AI 必需参数
	if APIService(apiSvc) == APIServiceVertexAI && projectValue == "" {
		fmt.Fprintln(os.Stderr, "错误: vertexai 模式需要指定 -j/--project 参数")
		os.Exit(1)
	}

	// 处理 prompt_file
	promptFileValue := promptFile
	if promptFileValue == "" && config != nil && prompt == "" {
		promptFileValue = config.PromptFile
	}

	// 验证 prompt_file 是否存在
	if promptFileValue != "" && !util.FileExists(promptFileValue) {
		fmt.Fprintf(os.Stderr, "错误: 提示词文件不存在: %s\n", promptFileValue)
		os.Exit(1)
	}

	// 合并图片列表
	if config != nil && len(config.Images) > 0 {
		images = append(config.Images, images...)
	}

	// 验证所有输入图片文件是否存在
	for _, imagePath := range images {
		if !util.FileExists(imagePath) {
			fmt.Fprintf(os.Stderr, "错误: 图片文件不存在: %s\n", imagePath)
			os.Exit(1)
		}
	}

	// 验证输出目录是否存在
	outputDir := getDir(outputValue)
	if outputDir != "" && !util.DirExists(outputDir) {
		fmt.Fprintf(os.Stderr, "错误: 输出目录不存在: %s\n", outputDir)
		os.Exit(1)
	}

	// 确定提示词
	var promptText string
	if promptFileValue != "" {
		data, err := os.ReadFile(promptFileValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读取提示词文件失败: %v\n", err)
			os.Exit(1)
		}
		promptText = string(data)
	} else if prompt != "" {
		promptText = prompt
	} else {
		promptText = getConfigValue("", getConfigString(config, "prompt"), Defaults.Prompt)
	}

	if strings.TrimSpace(promptText) == "" {
		fmt.Fprintln(os.Stderr, "错误: 提示词不能为空")
		os.Exit(1)
	}

	// 调用 API 生成图片
	var imageBytes []byte
	var textResponse string
	var err error

	ctx := context.Background()

	switch APIService(apiSvc) {
	case APIServiceOpenAI:
		client, clientErr := service.NewOpenAIClient(service.OpenAIConfig{
			APIKey:  apiKeyValue,
			BaseURL: baseURLValue,
			Model:   modelName,
		})
		if clientErr != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", clientErr)
			os.Exit(1)
		}
		imageBytes, textResponse, err = client.GenerateImageViaChat(modelName, promptText, images)

	case APIServiceGemini, APIServiceVertexAI:
		client, clientErr := service.NewGeminiClient(ctx, service.GeminiConfig{
			APIKey:      apiKeyValue,
			BaseURL:     baseURLValue,
			Vertex:      APIService(apiSvc) == APIServiceVertexAI,
			Project:     projectValue,
			Location:    locationValue,
			Credentials: credentialsValue,
		})
		if clientErr != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", clientErr)
			os.Exit(1)
		}
		defer client.Close()
		imageBytes, textResponse, err = client.GenerateImageViaChat(ctx, modelName, promptText, images, aspectRatioValue, resolutionValue)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	// 检查是否生成了图片
	if imageBytes == nil || len(imageBytes) == 0 {
		if textResponse != "" {
			maxLen := 500
			if len(textResponse) > maxLen {
				textResponse = textResponse[:maxLen] + "..."
			}
			fmt.Fprintf(os.Stderr, "错误: 响应中未找到图片数据。响应内容: %s\n", textResponse)
		} else {
			fmt.Fprintln(os.Stderr, "错误: 响应中未找到图片数据")
		}
		os.Exit(1)
	}

	// 保存图片
	if err := util.SaveImageBytes(imageBytes, outputValue); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 保存图片失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("图片已保存到: %s\n", outputValue)
}

// getConfigValue 获取配置值，优先级：CLI > 配置文件 > 默认值
func getConfigValue(cliValue, configValue, defaultValue string) string {
	if cliValue != "" {
		return cliValue
	}
	if configValue != "" {
		return configValue
	}
	return defaultValue
}

func getConfigValueWithSource(cliValue, configValue, defaultValue string) (string, string) {
	if cliValue != "" {
		return cliValue, "cli"
	}
	if configValue != "" {
		return configValue, "config"
	}
	return defaultValue, "default"
}

// getConfigString 从配置中获取字符串值
func getConfigString(config *Config, key string) string {
	if config == nil {
		return ""
	}
	switch key {
	case "api_service":
		return config.APIService
	case "model":
		return config.Model
	case "text_model":
		return config.TextModel
	case "base_url":
		return config.BaseURL
	case "api_key":
		return config.APIKey
	case "output":
		return config.Output
	case "prompt":
		return config.Prompt
	case "prompt_file":
		return config.PromptFile
	case "project":
		return config.Project
	case "location":
		return config.Location
	case "credentials":
		return config.Credentials
	case "aspect_ratio":
		return config.AspectRatio
	case "resolution":
		return config.Resolution
	default:
		return ""
	}
}

// getAPIServiceNames 获取所有 API 服务名称
func getAPIServiceNames() []string {
	services := ValidAPIServices()
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = string(s)
	}
	return names
}

// getDir 获取路径的目录部分
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}
