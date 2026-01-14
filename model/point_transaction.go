package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type PointTransactionType int

const (
	PointTransactionTypeUser PointTransactionType = iota
	PointTransactionTypeOrg
)

type PointReason string

const (
	PointReasonDailyLogin    PointReason = "daily_login"
	PointReasonReferralBonus PointReason = "referral_bonus"
	PointReasonReferredBonus PointReason = "referred_bonus"
	PointReasonImageGen      PointReason = "image_generation"
	PointReasonEnhancePrompt PointReason = "enhance_prompt"
	PointReasonRefund        PointReason = "refund"
	PointReasonAdminGrant    PointReason = "admin_grant"
	PointReasonOrgAllocation PointReason = "org_allocation"
	PointReasonOrgInitial    PointReason = "org_initial"
	PointReasonOrgAdjust     PointReason = "org_adjust"
	PointReasonOrgAllocOut   PointReason = "org_allocation_out"
)

type PointTransaction struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`

	Type           PointTransactionType `gorm:"not null" json:"type"`
	UserID         *uint                `gorm:"index:idx_pt_user_time" json:"user_id,omitempty"`
	OrganizationID *uint                `gorm:"index:idx_pt_org_time" json:"organization_id,omitempty"`

	Amount       int         `gorm:"not null" json:"amount"`
	Reason       PointReason `gorm:"size:30;not null;index:idx_pt_reason" json:"reason"`
	Description  string      `gorm:"size:200" json:"description,omitempty"`
	BalanceAfter int         `gorm:"not null" json:"balance_after"`

	OperatorID *uint `gorm:"index:idx_pt_operator" json:"operator_id,omitempty"`

	OperationID      string `gorm:"size:64;not null" json:"operation_id"`
	RefTransactionID *uint  `gorm:"uniqueIndex:idx_pt_ref_unique" json:"ref_transaction_id,omitempty"`

	RelatedOrgID  *uint `json:"related_org_id,omitempty"`
	RelatedUserID *uint `json:"related_user_id,omitempty"`
}

func (pt *PointTransaction) BeforeCreate(tx *gorm.DB) error {
	if pt.Type == PointTransactionTypeUser {
		if pt.UserID == nil {
			return errors.New("用户记录必须设置 UserID")
		}
		if pt.OrganizationID != nil {
			return errors.New("用户记录不能设置 OrganizationID")
		}
	} else if pt.Type == PointTransactionTypeOrg {
		if pt.OrganizationID == nil {
			return errors.New("组织记录必须设置 OrganizationID")
		}
		if pt.UserID != nil {
			return errors.New("组织记录不能设置 UserID")
		}
	}

	if pt.Reason == PointReasonOrgAllocation && (pt.RelatedOrgID == nil || *pt.RelatedOrgID == 0) {
		return errors.New("组织划拨记录必须设置 RelatedOrgID")
	}
	if pt.Reason == PointReasonOrgAllocOut && (pt.RelatedUserID == nil || *pt.RelatedUserID == 0) {
		return errors.New("组织划拨支出记录必须设置 RelatedUserID")
	}
	if pt.Reason == PointReasonRefund && (pt.RefTransactionID == nil || *pt.RefTransactionID == 0) {
		return errors.New("退款记录必须设置 RefTransactionID")
	}

	if pt.OperationID == "" {
		return errors.New("OperationID 不能为空")
	}

	return nil
}
