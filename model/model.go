package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type UserType int

const (
	UserTypeNormal     UserType = iota // 普通用户
	UserTypeSuperAdmin                 // 超级管理员
)

func (t UserType) String() string {
	switch t {
	case UserTypeSuperAdmin:
		return "超级管理员"
	default:
		return "普通用户"
	}
}

type MemberRole int

const (
	MemberRoleMember MemberRole = iota // 普通成员
	MemberRoleAdmin                    // 管理员
)

func (r MemberRole) String() string {
	switch r {
	case MemberRoleAdmin:
		return "管理员"
	default:
		return "成员"
	}
}

type User struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Email          string         `gorm:"uniqueIndex;size:100;not null" json:"email"`
	PasswordHash   string         `gorm:"size:60;not null" json:"-"`
	EmailVerified  bool           `gorm:"default:false" json:"email_verified"`
	Disabled       bool           `gorm:"default:false" json:"disabled"`
	Type           UserType       `gorm:"default:0" json:"type"`
	Points         int            `gorm:"default:0" json:"points"`
	LastPointsDate *time.Time     `gorm:"index" json:"-"`
	Memberships    []Membership   `gorm:"foreignKey:UserID" json:"memberships,omitempty"`
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
	Token     string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
	Used      bool      `gorm:"default:false" json:"-"`
}

type EmailVerificationToken struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"-"`
	Token     string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
	Used      bool      `gorm:"default:false" json:"-"`
}

type Organization struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"uniqueIndex;size:100;not null" json:"name"`
	Points    int            `gorm:"default:0" json:"points"`
	Members   []Membership   `gorm:"foreignKey:OrganizationID" json:"members,omitempty"`
}

type Membership struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	UserID         uint           `gorm:"uniqueIndex:idx_user_org;not null" json:"user_id"`
	OrganizationID uint           `gorm:"uniqueIndex:idx_user_org;not null" json:"organization_id"`
	Role           MemberRole     `gorm:"default:0" json:"role"`
	User           *User          `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Organization   *Organization  `gorm:"foreignKey:OrganizationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func InitDB(dbType, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch dbType {
	case "sqlite":
		if !strings.Contains(dsn, "?") {
			dsn += "?_pragma=foreign_keys(1)"
		} else if !strings.Contains(dsn, "_pragma=foreign_keys") {
			dsn += "&_pragma=foreign_keys(1)"
		}
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

	if err := db.AutoMigrate(&User{}, &Session{}, &PasswordResetToken{}, &EmailVerificationToken{}, &Organization{}, &Membership{}); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return db, nil
}
