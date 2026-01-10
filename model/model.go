package model

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Email         string         `gorm:"uniqueIndex;size:100;not null" json:"email"`
	PasswordHash  string         `gorm:"size:60;not null" json:"-"`
	EmailVerified bool           `gorm:"default:false" json:"email_verified"`
}

type Session struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"-"`
	Token     string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
}

type PasswordResetToken struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"-"`
	Token     string    `gorm:"uniqueIndex;size:64;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	Used      bool      `gorm:"default:false"`
}

type EmailVerificationToken struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"-"`
	Token     string    `gorm:"uniqueIndex;size:64;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	Used      bool      `gorm:"default:false"`
}

func InitDB(dbType, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch dbType {
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "mysql":
		if dsn == "" {
			return nil, fmt.Errorf("MySQL 需要指定连接字符串 (--db-dsn)")
		}
		dialector = mysql.Open(dsn)
	case "postgres":
		if dsn == "" {
			return nil, fmt.Errorf("PostgreSQL 需要指定连接字符串 (--db-dsn)")
		}
		dialector = postgres.Open(dsn)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("无法连接数据库: %w", err)
	}

	if err := db.AutoMigrate(&User{}, &Session{}, &PasswordResetToken{}, &EmailVerificationToken{}); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return db, nil
}
