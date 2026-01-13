package model

import (
	"time"
)

type IdempotencyKey struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `gorm:"index"`

	UserID uint   `gorm:"uniqueIndex:idx_user_key;not null"`
	Key    string `gorm:"uniqueIndex:idx_user_key;size:64;not null"`

	Method   string `gorm:"size:10;not null"`
	Path     string `gorm:"size:255;not null"`
	BodyHash string `gorm:"size:64"`

	Status string `gorm:"size:20;not null;default:'pending'"`

	ResponseStatus  int    `gorm:"default:0"`
	ResponseHeaders string `gorm:"type:text"`
	ResponseBody    []byte `gorm:"type:longblob"`

	ExpiresAt time.Time `gorm:"index;not null"`
}
