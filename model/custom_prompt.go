package model

import (
	"regexp"
	"time"
)

type CustomPrompt struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_client;not null" json:"-"`
	ClientID  string    `gorm:"uniqueIndex:idx_user_client;size:36;not null" json:"client_id"`
	Title     string    `gorm:"size:400;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
