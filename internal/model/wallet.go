package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WalletType string

const (
	WalletTypeCash    WalletType = "CASH"
	WalletTypeBank    WalletType = "BANK"
	WalletTypeEWallet WalletType = "E_WALLET"
	WalletTypeSavings WalletType = "SAVINGS"
	WalletTypeCredit  WalletType = "CREDIT"
	WalletTypeJar     WalletType = "JAR"
)

type AllocationPreset string

const (
	AllocationPreset503020 AllocationPreset = "50_30_20"
	AllocationPreset6Jars  AllocationPreset = "6_JARS"
	AllocationPresetCustom AllocationPreset = "CUSTOM"
)

type Wallet struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"userId"`
	Name              string     `gorm:"type:varchar(100);not null" json:"name"`
	Type              WalletType `gorm:"type:varchar(20);not null;default:'CASH'" json:"type"`
	Balance           float64    `gorm:"type:numeric(15,2);not null;default:0.00" json:"balance"`
	Currency          string     `gorm:"type:varchar(10);default:'VND'" json:"currency"`
	Icon              string     `gorm:"type:varchar(50)" json:"icon"`
	JarCategory       *string    `gorm:"type:varchar(50)" json:"jarCategory,omitempty"`
	AllocationPercent float64    `gorm:"type:numeric(5,2);default:0.00" json:"allocationPercent"`
	IsIncludedInTotal bool       `gorm:"type:boolean;default:true" json:"isIncludedInTotal"`
	CreatedAt         time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
	DeletedAt         DeletedAt  `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type WalletTransfer struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	FromWalletID uuid.UUID `gorm:"type:uuid;not null;index" json:"fromWalletId"`
	ToWalletID   uuid.UUID `gorm:"type:uuid;not null;index" json:"toWalletId"`
	Amount       float64   `gorm:"type:numeric(15,2);not null" json:"amount"`
	Fee          float64   `gorm:"type:numeric(15,2);default:0.00" json:"fee"`
	Note         *string   `gorm:"type:text" json:"note,omitempty"`
	TransferDate time.Time `gorm:"type:date;not null" json:"transferDate"`
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`

	FromWallet *Wallet `gorm:"foreignKey:FromWalletID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	ToWallet   *Wallet `gorm:"foreignKey:ToWalletID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (w *Wallet) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == uuid.Nil {
		w.ID, err = uuid.NewV7()
	}
	return err
}

func (wt *WalletTransfer) BeforeCreate(tx *gorm.DB) (err error) {
	if wt.ID == uuid.Nil {
		wt.ID, err = uuid.NewV7()
	}
	return err
}
