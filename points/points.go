package points

import (
	"errors"
	"fmt"
	"strings"

	"genimage/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInsufficientPoints = errors.New("积分不足")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrOrgNotFound        = errors.New("组织不存在")
	ErrInvalidAmount      = errors.New("金额无效")
	ErrDuplicateOperation = errors.New("重复操作")
)

func mapDuplicateOperationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicateOperation
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "unique") {
		return ErrDuplicateOperation
	}
	return err
}

type AddUserPointsParams struct {
	UserID      uint
	Amount      int
	Reason      model.PointReason
	Description string
	OperatorID  *uint
	OperationID string
}

func AddUserPointsTx(tx *gorm.DB, p AddUserPointsParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, p.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	newBalance := user.Points + p.Amount
	if err := tx.Model(&user).Update("points", newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:         model.PointTransactionTypeUser,
		UserID:       &p.UserID,
		Amount:       p.Amount,
		Reason:       p.Reason,
		Description:  p.Description,
		BalanceAfter: newBalance,
		OperatorID:   p.OperatorID,
		OperationID:  p.OperationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func AddUserPoints(db *gorm.DB, p AddUserPointsParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = AddUserPointsTx(tx, p)
		return e
	})
	return record, err
}

type DeductUserPointsParams struct {
	UserID      uint
	Amount      int
	Reason      model.PointReason
	Description string
	OperationID string
}

func DeductUserPointsTx(tx *gorm.DB, p DeductUserPointsParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	result := tx.Model(&model.User{}).
		Where("id = ? AND points >= ?", p.UserID, p.Amount).
		Update("points", gorm.Expr("points - ?", p.Amount))

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		var exists bool
		if err := tx.Model(&model.User{}).Select("1").Where("id = ?", p.UserID).Scan(&exists).Error; err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrUserNotFound
		}
		return nil, ErrInsufficientPoints
	}

	var newBalance int
	if err := tx.Model(&model.User{}).Select("points").Where("id = ?", p.UserID).Scan(&newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:         model.PointTransactionTypeUser,
		UserID:       &p.UserID,
		Amount:       -p.Amount,
		Reason:       p.Reason,
		Description:  p.Description,
		BalanceAfter: newBalance,
		OperationID:  p.OperationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func DeductUserPoints(db *gorm.DB, p DeductUserPointsParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = DeductUserPointsTx(tx, p)
		return e
	})
	return record, err
}

type RefundParams struct {
	UserID           uint
	Amount           int
	Description      string
	OperationID      string
	RefTransactionID uint
}

func RefundUserPointsTx(tx *gorm.DB, p RefundParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, p.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	newBalance := user.Points + p.Amount
	if err := tx.Model(&user).Update("points", newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:             model.PointTransactionTypeUser,
		UserID:           &p.UserID,
		Amount:           p.Amount,
		Reason:           model.PointReasonRefund,
		Description:      p.Description,
		BalanceAfter:     newBalance,
		OperationID:      p.OperationID,
		RefTransactionID: &p.RefTransactionID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func RefundUserPoints(db *gorm.DB, p RefundParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = RefundUserPointsTx(tx, p)
		return e
	})
	return record, err
}

type AddOrgPointsParams struct {
	OrgID       uint
	Amount      int
	Reason      model.PointReason
	Description string
	OperatorID  *uint
	OperationID string
}

func AddOrgPointsTx(tx *gorm.DB, p AddOrgPointsParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	var org model.Organization
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&org, p.OrgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}

	newBalance := org.Points + p.Amount
	if err := tx.Model(&org).Update("points", newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:           model.PointTransactionTypeOrg,
		OrganizationID: &p.OrgID,
		Amount:         p.Amount,
		Reason:         p.Reason,
		Description:    p.Description,
		BalanceAfter:   newBalance,
		OperatorID:     p.OperatorID,
		OperationID:    p.OperationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func AddOrgPoints(db *gorm.DB, p AddOrgPointsParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = AddOrgPointsTx(tx, p)
		return e
	})
	return record, err
}

type DeductOrgPointsParams struct {
	OrgID       uint
	Amount      int
	Reason      model.PointReason
	Description string
	OperationID string
}

func DeductOrgPointsTx(tx *gorm.DB, p DeductOrgPointsParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	result := tx.Model(&model.Organization{}).
		Where("id = ? AND points >= ?", p.OrgID, p.Amount).
		Update("points", gorm.Expr("points - ?", p.Amount))

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		var exists bool
		if err := tx.Model(&model.Organization{}).Select("1").Where("id = ?", p.OrgID).Scan(&exists).Error; err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrOrgNotFound
		}
		return nil, ErrInsufficientPoints
	}

	var newBalance int
	if err := tx.Model(&model.Organization{}).Select("points").Where("id = ?", p.OrgID).Scan(&newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:           model.PointTransactionTypeOrg,
		OrganizationID: &p.OrgID,
		Amount:         -p.Amount,
		Reason:         p.Reason,
		Description:    p.Description,
		BalanceAfter:   newBalance,
		OperationID:    p.OperationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func DeductOrgPoints(db *gorm.DB, p DeductOrgPointsParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = DeductOrgPointsTx(tx, p)
		return e
	})
	return record, err
}

type SetOrgPointsParams struct {
	OrgID       uint
	NewBalance  int
	Reason      model.PointReason
	Description string
	OperatorID  *uint
	OperationID string
}

func SetOrgPointsTx(tx *gorm.DB, p SetOrgPointsParams) (*model.PointTransaction, error) {
	if p.NewBalance < 0 {
		return nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	var org model.Organization
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&org, p.OrgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}

	delta := p.NewBalance - org.Points
	if delta == 0 {
		return nil, nil
	}

	if err := tx.Model(&org).Update("points", p.NewBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:           model.PointTransactionTypeOrg,
		OrganizationID: &p.OrgID,
		Amount:         delta,
		Reason:         p.Reason,
		Description:    p.Description,
		BalanceAfter:   p.NewBalance,
		OperatorID:     p.OperatorID,
		OperationID:    p.OperationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func SetOrgPoints(db *gorm.DB, p SetOrgPointsParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = SetOrgPointsTx(tx, p)
		return e
	})
	return record, err
}

type TransferOrgToUserParams struct {
	OrgID       uint
	UserID      uint
	Amount      int
	Description string
	OperatorID  *uint
	OperationID string
}

func TransferOrgToUserTx(tx *gorm.DB, p TransferOrgToUserParams) (*model.PointTransaction, *model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, nil, ErrInvalidAmount
	}

	if p.OperationID == "" {
		p.OperationID = uuid.New().String()
	}

	result := tx.Model(&model.Organization{}).
		Where("id = ? AND points >= ?", p.OrgID, p.Amount).
		Update("points", gorm.Expr("points - ?", p.Amount))

	if result.Error != nil {
		return nil, nil, result.Error
	}
	if result.RowsAffected == 0 {
		var exists bool
		if err := tx.Model(&model.Organization{}).Select("1").Where("id = ?", p.OrgID).Scan(&exists).Error; err != nil {
			return nil, nil, err
		}
		if !exists {
			return nil, nil, ErrOrgNotFound
		}
		return nil, nil, ErrInsufficientPoints
	}

	var orgBalance int
	if err := tx.Model(&model.Organization{}).Select("points").Where("id = ?", p.OrgID).Scan(&orgBalance).Error; err != nil {
		return nil, nil, err
	}

	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, p.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrUserNotFound
		}
		return nil, nil, err
	}

	userBalance := user.Points + p.Amount
	if err := tx.Model(&user).Update("points", userBalance).Error; err != nil {
		return nil, nil, err
	}

	orgRecord := &model.PointTransaction{
		Type:           model.PointTransactionTypeOrg,
		OrganizationID: &p.OrgID,
		Amount:         -p.Amount,
		Reason:         model.PointReasonOrgAllocOut,
		Description:    p.Description,
		BalanceAfter:   orgBalance,
		OperatorID:     p.OperatorID,
		OperationID:    p.OperationID,
		RelatedUserID:  &p.UserID,
	}

	if err := tx.Create(orgRecord).Error; err != nil {
		return nil, nil, mapDuplicateOperationError(err)
	}

	userRecord := &model.PointTransaction{
		Type:         model.PointTransactionTypeUser,
		UserID:       &p.UserID,
		Amount:       p.Amount,
		Reason:       model.PointReasonOrgAllocation,
		Description:  p.Description,
		BalanceAfter: userBalance,
		OperatorID:   p.OperatorID,
		OperationID:  p.OperationID,
		RelatedOrgID: &p.OrgID,
	}

	if err := tx.Create(userRecord).Error; err != nil {
		return nil, nil, mapDuplicateOperationError(err)
	}

	return orgRecord, userRecord, nil
}

func TransferOrgToUser(db *gorm.DB, p TransferOrgToUserParams) (*model.PointTransaction, *model.PointTransaction, error) {
	var orgRecord, userRecord *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		orgRecord, userRecord, e = TransferOrgToUserTx(tx, p)
		return e
	})
	return orgRecord, userRecord, err
}

type AwardDailyLoginParams struct {
	UserID uint
	Amount int
	Date   string
}

func AwardDailyLoginPointsTx(tx *gorm.DB, p AwardDailyLoginParams) (*model.PointTransaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	operationID := fmt.Sprintf("daily_login:%d:%s", p.UserID, p.Date)

	result := tx.Model(&model.User{}).
		Where("id = ? AND (last_points_date IS NULL OR last_points_date < ?)", p.UserID, p.Date).
		Updates(map[string]interface{}{
			"points":           gorm.Expr("points + ?", p.Amount),
			"last_points_date": p.Date,
		})

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		var exists bool
		if err := tx.Model(&model.User{}).Select("1").Where("id = ?", p.UserID).Scan(&exists).Error; err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrUserNotFound
		}
		return nil, nil
	}

	var newBalance int
	if err := tx.Model(&model.User{}).Select("points").Where("id = ?", p.UserID).Scan(&newBalance).Error; err != nil {
		return nil, err
	}

	record := &model.PointTransaction{
		Type:         model.PointTransactionTypeUser,
		UserID:       &p.UserID,
		Amount:       p.Amount,
		Reason:       model.PointReasonDailyLogin,
		BalanceAfter: newBalance,
		OperationID:  operationID,
	}

	if err := tx.Create(record).Error; err != nil {
		return nil, mapDuplicateOperationError(err)
	}

	return record, nil
}

func AwardDailyLoginPoints(db *gorm.DB, p AwardDailyLoginParams) (*model.PointTransaction, error) {
	var record *model.PointTransaction
	err := db.Transaction(func(tx *gorm.DB) error {
		var e error
		record, e = AwardDailyLoginPointsTx(tx, p)
		return e
	})
	return record, err
}
