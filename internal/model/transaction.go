package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TransactionType string

const (
	TransactionTypeIncome     TransactionType = "INCOME"
	TransactionTypeExpense    TransactionType = "EXPENSE"
	TransactionTypeInvestment TransactionType = "INVESTMENT"
)

type InvestmentStatus string

const (
	InvestmentStatusHolding   InvestmentStatus = "HOLDING"
	InvestmentStatusSold      InvestmentStatus = "SOLD"
	InvestmentStatusMatured   InvestmentStatus = "MATURED"
	InvestmentStatusCancelled InvestmentStatus = "CANCELLED"
)

type Transaction struct {
	ID          uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID         `gorm:"type:uuid;not null;index" json:"userId"`
	CategoryID  uuid.UUID         `gorm:"type:uuid;not null;index" json:"categoryId"`
	WalletID    *uuid.UUID        `gorm:"type:uuid;index" json:"walletId,omitempty"`
	Amount      float64           `gorm:"type:numeric(15,2);not null" json:"amount"`
	Type        TransactionType   `gorm:"type:varchar(10);not null;check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')" json:"type"`
	Status      *InvestmentStatus `gorm:"type:varchar(20);default:'HOLDING'" json:"status,omitempty"`
	RealizedPnL *float64          `gorm:"column:realized_pnl;type:numeric(15,2);default:0.00" json:"realizedPnl,omitempty"`
	Description *string           `gorm:"type:text" json:"description,omitempty"`
	ReceiptURL  *string           `gorm:"type:varchar(1024)" json:"receiptUrl,omitempty"`
	// BalanceApplied is true when WalletBalanceEffect() has actually been applied
	// to the wallet. Rows created before wallet effects existed stay false, so
	// editing/deleting them never reverses an effect that was never applied.
	BalanceApplied  bool      `gorm:"not null;default:false" json:"-"`
	TransactionDate time.Time `gorm:"type:date;not null;index:idx_user_transaction_date" json:"transactionDate"`
	CreatedAt       time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt       DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User     *User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Wallet   *Wallet   `gorm:"foreignKey:WalletID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

// BeforeCreate GORM Hook to generate UUID v7
func (t *Transaction) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID, err = uuid.NewV7()
	}
	return err
}

// WalletBalanceEffect returns the signed amount this transaction moves in the
// balance of its wallet (only applied when WalletID is set).
//
// Rules, consistent with report_service investment semantics (HOLDING = money
// still invested; a closed position realises realizedPnl):
//   - INCOME:                                  +amount
//   - EXPENSE:                                 -amount
//   - INVESTMENT, status HOLDING or empty:     -amount (cash left the wallet)
//   - INVESTMENT, status SOLD/MATURED/CANCELLED: -amount + (amount + realizedPnl)
//     = +realizedPnl (the principal came back together with the realised P/L)
//
// Updates reverse the old effect (only if BalanceApplied) and apply the new one;
// deletes reverse it (only if BalanceApplied).
func (t *Transaction) WalletBalanceEffect() float64 {
	switch t.Type {
	case TransactionTypeIncome:
		return t.Amount
	case TransactionTypeExpense:
		return -t.Amount
	case TransactionTypeInvestment:
		if t.Status == nil || *t.Status == InvestmentStatusHolding || *t.Status == "" {
			return -t.Amount
		}
		pnl := 0.0
		if t.RealizedPnL != nil {
			pnl = *t.RealizedPnL
		}
		return pnl
	}
	return 0
}
