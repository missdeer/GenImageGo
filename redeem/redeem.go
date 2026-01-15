package redeem

import (
	"errors"
	"fmt"
	"time"

	"genimage/model"
	"genimage/points"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCodeNotFound    = errors.New("兑换码不存在或已使用")
	ErrCodeDisabled    = errors.New("兑换码已禁用")
	ErrInvalidPoints   = errors.New("积分值必须在1-100000之间")
	ErrInvalidCount    = errors.New("生成数量必须在1-1000之间")
	ErrTotalExceeded   = errors.New("单次生成总额不能超过1000000积分")
	ErrOrgNotFound     = errors.New("组织不存在")
	ErrNotOrgAdmin     = errors.New("您不是该组织的管理员")
	ErrCodeAlreadyUsed = errors.New("兑换码已被使用")
)

type GenerateParams struct {
	Points    int
	Count     int
	Note      string
	CreatorID uint
}

func GenerateCodes(db *gorm.DB, params GenerateParams) ([]model.RedeemCode, error) {
	if params.Points <= 0 || params.Points > 100000 {
		return nil, ErrInvalidPoints
	}
	if params.Count <= 0 || params.Count > 1000 {
		return nil, ErrInvalidCount
	}
	if params.Points*params.Count > 1000000 {
		return nil, ErrTotalExceeded
	}

	codes := make([]model.RedeemCode, params.Count)
	for i := 0; i < params.Count; i++ {
		codes[i] = model.RedeemCode{
			Points:    params.Points,
			CreatorID: params.CreatorID,
			Note:      params.Note,
			Status:    model.RedeemCodeStatusActive,
		}
	}

	if err := db.Create(&codes).Error; err != nil {
		return nil, err
	}

	return codes, nil
}

type RedeemParams struct {
	Code        string
	UserID      uint
	TargetType  model.RedeemCodeTargetType
	TargetOrgID uint
}

type RedeemResult struct {
	Points     int
	NewBalance int
	TargetType model.RedeemCodeTargetType
}

func RedeemCode(db *gorm.DB, params RedeemParams) (*RedeemResult, error) {
	normalizedCode := model.NormalizeRedeemCode(params.Code)

	var result *RedeemResult

	err := db.Transaction(func(tx *gorm.DB) error {
		var code model.RedeemCode
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ?", normalizedCode).
			First(&code).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCodeNotFound
			}
			return err
		}

		if code.Status == model.RedeemCodeStatusRedeemed {
			return ErrCodeAlreadyUsed
		}
		if code.Status == model.RedeemCodeStatusDisabled {
			return ErrCodeDisabled
		}

		now := time.Now()
		targetType := params.TargetType

		code.Status = model.RedeemCodeStatusRedeemed
		code.RedeemedAt = &now
		code.RedeemedByUserID = &params.UserID
		code.TargetType = &targetType

		operationID := fmt.Sprintf("redeem:%d", code.ID)
		var newBalance int

		if params.TargetType == model.RedeemCodeTargetUser {
			code.TargetUserID = &params.UserID

			record, err := points.AddUserPointsTx(tx, points.AddUserPointsParams{
				UserID:      params.UserID,
				Amount:      code.Points,
				Reason:      model.PointReasonRedeemCode,
				Description: fmt.Sprintf("兑换码: %s", code.Code),
				OperationID: operationID,
			})
			if err != nil {
				return err
			}
			newBalance = record.BalanceAfter
		} else {
			code.TargetOrgID = &params.TargetOrgID

			record, err := points.AddOrgPointsTx(tx, points.AddOrgPointsParams{
				OrgID:       params.TargetOrgID,
				Amount:      code.Points,
				Reason:      model.PointReasonRedeemCodeOrg,
				Description: fmt.Sprintf("兑换码: %s", code.Code),
				OperatorID:  &params.UserID,
				OperationID: operationID,
			})
			if err != nil {
				return err
			}
			newBalance = record.BalanceAfter
		}

		if err := tx.Save(&code).Error; err != nil {
			return err
		}

		result = &RedeemResult{
			Points:     code.Points,
			NewBalance: newBalance,
			TargetType: params.TargetType,
		}

		return nil
	})

	return result, err
}

func DisableCode(db *gorm.DB, codeID uint) error {
	result := db.Model(&model.RedeemCode{}).
		Where("id = ? AND status = ?", codeID, model.RedeemCodeStatusActive).
		Update("status", model.RedeemCodeStatusDisabled)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCodeNotFound
	}
	return nil
}

func EnableCode(db *gorm.DB, codeID uint) error {
	result := db.Model(&model.RedeemCode{}).
		Where("id = ? AND status = ?", codeID, model.RedeemCodeStatusDisabled).
		Update("status", model.RedeemCodeStatusActive)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCodeNotFound
	}
	return nil
}

func IsOrgAdmin(db *gorm.DB, userID, orgID uint) (bool, error) {
	var count int64
	err := db.Model(&model.Membership{}).
		Where("user_id = ? AND organization_id = ? AND role = ?", userID, orgID, model.MemberRoleAdmin).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
