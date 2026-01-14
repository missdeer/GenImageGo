package siteconfig

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"genimage/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db        *gorm.DB
	cache     map[string]string
	overrides map[string]int // CLI/config overrides (memory only, not persisted)
	mu        sync.RWMutex
}

type ConfigItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type ConfigUpdate struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var knownKeys map[string]bool

func init() {
	knownKeys = make(map[string]bool, len(model.DefaultSiteConfigs))
	for _, c := range model.DefaultSiteConfigs {
		knownKeys[c.Key] = true
	}
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:        db,
		cache:     make(map[string]string),
		overrides: make(map[string]int),
	}
}

func (s *Service) Load() error {
	if err := s.ensureDefaults(); err != nil {
		return fmt.Errorf("确保默认配置失败: %w", err)
	}

	var configs []model.SiteConfig
	if err := s.db.Find(&configs).Error; err != nil {
		return fmt.Errorf("加载站点配置失败: %w", err)
	}

	s.mu.Lock()
	newCache := make(map[string]string, len(configs))
	for _, c := range configs {
		newCache[c.Key] = c.Value
	}
	s.cache = newCache
	s.mu.Unlock()

	log.Printf("siteconfig: loaded %d configs", len(configs))
	return nil
}

func (s *Service) ensureDefaults() error {
	for _, def := range model.DefaultSiteConfigs {
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&def).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Get(key string) string {
	s.mu.RLock()
	val := s.cache[key]
	s.mu.RUnlock()
	return val
}

func (s *Service) GetInt(key string, defaultVal int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check memory overrides first (CLI/config values take priority)
	if override, ok := s.overrides[key]; ok {
		return override
	}

	// Fall back to database cache
	val, ok := s.cache[key]
	if !ok {
		log.Printf("siteconfig: key %q not found, using default %d", key, defaultVal)
		return defaultVal
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("siteconfig: key %q value %q parse failed, using default %d", key, val, defaultVal)
		return defaultVal
	}
	return n
}

func (s *Service) GetAll() []ConfigItem {
	var configs []model.SiteConfig
	if err := s.db.Order("key").Find(&configs).Error; err != nil {
		log.Printf("siteconfig: GetAll failed: %v", err)
		return nil
	}

	items := make([]ConfigItem, len(configs))
	for i, c := range configs {
		items[i] = ConfigItem{
			Key:   c.Key,
			Value: c.Value,
			Label: c.Label,
			Type:  c.Type,
		}
	}
	return items
}

func (s *Service) SetBatch(items []ConfigUpdate) error {
	// Normalize values first
	for i := range items {
		items[i].Value = strings.TrimSpace(items[i].Value)
	}

	for _, item := range items {
		if !knownKeys[item.Key] {
			return fmt.Errorf("未知的配置项: %s", item.Key)
		}
		if err := s.validateValue(item.Key, item.Value); err != nil {
			return fmt.Errorf("配置项 %s 值无效: %w", item.Key, err)
		}
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Model(&model.SiteConfig{}).
				Where("key = ?", item.Key).
				Updates(map[string]interface{}{"value": item.Value})
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	for _, item := range items {
		s.cache[item.Key] = item.Value
	}
	s.mu.Unlock()

	log.Printf("siteconfig: updated %d configs", len(items))
	return nil
}

func (s *Service) validateValue(key, value string) error {
	value = strings.TrimSpace(value)
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("必须为整数")
	}
	if n < 0 || n > 1000 {
		return fmt.Errorf("必须为 0-1000 的整数")
	}
	return nil
}

type OverrideConfig struct {
	DailyLoginPoints      *int
	ImageGenerationPoints *int
	EnhancePromptPoints   *int
}

func (s *Service) SetOverrides(cfg OverrideConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.DailyLoginPoints != nil {
		s.overrides[model.ConfigKeyDailyLoginPoints] = *cfg.DailyLoginPoints
		log.Printf("siteconfig: override %s = %d", model.ConfigKeyDailyLoginPoints, *cfg.DailyLoginPoints)
	}
	if cfg.ImageGenerationPoints != nil {
		s.overrides[model.ConfigKeyImageGenerationPoints] = *cfg.ImageGenerationPoints
		log.Printf("siteconfig: override %s = %d", model.ConfigKeyImageGenerationPoints, *cfg.ImageGenerationPoints)
	}
	if cfg.EnhancePromptPoints != nil {
		s.overrides[model.ConfigKeyEnhancePromptPoints] = *cfg.EnhancePromptPoints
		log.Printf("siteconfig: override %s = %d", model.ConfigKeyEnhancePromptPoints, *cfg.EnhancePromptPoints)
	}
}
