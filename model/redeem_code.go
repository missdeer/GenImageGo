package model

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RedeemCodeStatus int

const (
	RedeemCodeStatusActive   RedeemCodeStatus = iota // 未使用
	RedeemCodeStatusRedeemed                         // 已兑换
	RedeemCodeStatusDisabled                         // 已禁用
)

func (s RedeemCodeStatus) String() string {
	switch s {
	case RedeemCodeStatusRedeemed:
		return "已兑换"
	case RedeemCodeStatusDisabled:
		return "已禁用"
	default:
		return "未使用"
	}
}

type RedeemCodeTargetType int

const (
	RedeemCodeTargetUser RedeemCodeTargetType = iota // 兑换到用户
	RedeemCodeTargetOrg                              // 兑换到组织
)

type RedeemCode struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Code   string           `gorm:"uniqueIndex;size:32;not null" json:"code"`
	Points int              `gorm:"not null" json:"points"`
	Status RedeemCodeStatus `gorm:"default:0;index" json:"status"`

	CreatorID uint   `gorm:"index;not null" json:"creator_id"`
	Note      string `gorm:"size:200" json:"note,omitempty"`

	RedeemedAt       *time.Time            `json:"redeemed_at,omitempty"`
	RedeemedByUserID *uint                 `gorm:"index" json:"redeemed_by_user_id,omitempty"`
	TargetType       *RedeemCodeTargetType `json:"target_type,omitempty"`
	TargetUserID     *uint                 `json:"target_user_id,omitempty"`
	TargetOrgID      *uint                 `json:"target_org_id,omitempty"`
}

func GenerateRedeemCode() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	return FormatRedeemCode(encoded[:24]), nil
}

func FormatRedeemCode(s string) string {
	s = strings.ToUpper(s)
	var parts []string
	for i := 0; i < len(s); i += 4 {
		end := i + 4
		if end > len(s) {
			end = len(s)
		}
		parts = append(parts, s[i:end])
	}
	return strings.Join(parts, "-")
}

func NormalizeRedeemCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 24 {
		return FormatRedeemCode(code)
	}
	return code
}

func (rc *RedeemCode) BeforeCreate(tx *gorm.DB) error {
	if rc.Code == "" {
		code, err := GenerateRedeemCode()
		if err != nil {
			return err
		}
		rc.Code = code
	}
	return nil
}
