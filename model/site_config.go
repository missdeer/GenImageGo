package model

import (
	"time"
)

type SiteConfig struct {
	Key       string    `gorm:"primaryKey;size:50;not null" json:"key"`
	Value     string    `gorm:"size:500;not null" json:"value"`
	Label     string    `gorm:"size:100;not null" json:"label"`
	Type      string    `gorm:"size:20;not null;default:'int'" json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	ConfigKeyDailyLoginPoints      = "daily_login_points"
	ConfigKeyImageGenerationPoints = "image_generation_points"
	ConfigKeyEnhancePromptPoints   = "enhance_prompt_points"
)

var DefaultSiteConfigs = []SiteConfig{
	{Key: ConfigKeyDailyLoginPoints, Value: "10", Label: "每日登录奖励积分", Type: "int"},
	{Key: ConfigKeyImageGenerationPoints, Value: "20", Label: "生图扣除积分", Type: "int"},
	{Key: ConfigKeyEnhancePromptPoints, Value: "4", Label: "提示词优化积分", Type: "int"},
}
