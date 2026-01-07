package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/bmp"
)

// GetMIMEType 根据文件扩展名获取 MIME 类型
func GetMIMEType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeTypes := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
	}
	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}
	return "image/jpeg"
}

// EncodeImageToBase64 将图片文件编码为 base64 字符串
func EncodeImageToBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("无法读取图片文件: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// ConvertImageToPNG 将 BMP 图片转换为 PNG 格式并返回 base64 编码
func ConvertImageToPNG(imagePath string) (string, string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("打开图片失败: %w", err)
	}
	defer file.Close()

	img, err := bmp.Decode(file)
	if err != nil {
		return "", "", fmt.Errorf("解码 BMP 图片失败: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", fmt.Errorf("编码为 PNG 失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), "image/png", nil
}

// EncodeImageForAPI 将图片文件编码为 base64，并返回正确的 MIME 类型
// 对于 BMP 格式会先转换为 PNG
func EncodeImageForAPI(imagePath string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(imagePath))

	// BMP 需要转换为 PNG
	if ext == ".bmp" {
		return ConvertImageToPNG(imagePath)
	}

	// 标准图片格式直接编码
	b64, err := EncodeImageToBase64(imagePath)
	if err != nil {
		return "", "", err
	}

	return b64, GetMIMEType(imagePath), nil
}

// LoadImageAsBase64 加载图片并返回 data URI 格式
func LoadImageAsBase64(imagePath string) (string, error) {
	b64, mimeType, err := EncodeImageForAPI(imagePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64), nil
}

// SaveImageBytes 保存图片字节到文件
func SaveImageBytes(data []byte, outputPath string) error {
	return os.WriteFile(outputPath, data, 0644)
}

// DecodeImage 从字节数据解码图片
func DecodeImage(data []byte) (image.Image, string, error) {
	reader := bytes.NewReader(data)
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败: %w", err)
	}
	return img, format, nil
}

// EncodeImageToBytes 将图片编码为指定格式的字节
func EncodeImageToBytes(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("编码为 PNG 失败: %w", err)
		}
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("编码为 JPEG 失败: %w", err)
		}
	default:
		// 默认使用 PNG
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("编码为 PNG 失败: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// DirExists 检查目录是否存在
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}
