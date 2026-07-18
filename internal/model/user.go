package model

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username" validate:"required,min=3,max=50"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email" validate:"required,email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	Role      string    `gorm:"type:varchar(20);default:'user';not null" json:"role"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt,omitempty"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt,omitempty"`
	DeletedAt DeletedAt `gorm:"index" json:"deletedAt,omitempty" swaggertype:"string"`
}

// BeforeCreate GORM Hook to generate UUID v7
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV7()
	}
	return err
}

// Validate validates the User struct
func (u *User) Validate() error {
	validate := validator.New()
	return validate.Struct(u)
}

// HashPassword hashes the user's password and stores it in the Password field
func (u *User) HashPassword() error {
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

// CheckPassword checks if the provided password matches the hashed password
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

// AfterCreate GORM Hook to automatically seed default categories for a new user
func (u *User) AfterCreate(tx *gorm.DB) error {
	defaultCategories := []Category{
		{Name: "Ăn uống", Type: CategoryTypeExpense, Description: strPtr("Chi tiêu cho thực phẩm, ăn uống hàng ngày")},
		{Name: "Di chuyển", Type: CategoryTypeExpense, Description: strPtr("Chi phí đi lại, xăng xe, dịch vụ vận chuyển")},
		{Name: "Nhà cửa", Type: CategoryTypeExpense, Description: strPtr("Tiền thuê nhà, hóa đơn điện nước và dịch vụ")},
		{Name: "Giải trí", Type: CategoryTypeExpense, Description: strPtr("Xem phim, mua sắm giải trí, du lịch")},
		{Name: "Y tế & Sức khỏe", Type: CategoryTypeExpense, Description: strPtr("Khám bệnh, thuốc men, tập thể dục")},
		{Name: "Giáo dục", Type: CategoryTypeExpense, Description: strPtr("Học phí, sách vở, khóa học phát triển")},
		{Name: "Tiền nhà", Type: CategoryTypeExpense, Description: strPtr("Chi phí thuê nhà, tiền nhà hàng tháng")},
		{Name: "Tiền cầu lông", Type: CategoryTypeExpense, Description: strPtr("Chi phí chơi cầu lông, sân bãi, dụng cụ")},
		{Name: "Tiền hẹn hò", Type: CategoryTypeExpense, Description: strPtr("Chi phí hẹn hò, ăn uống giải trí cùng đối phương")},
		{Name: "Lương", Type: CategoryTypeIncome, Description: strPtr("Thu nhập từ lương công việc chính")},
		{Name: "Thưởng", Type: CategoryTypeIncome, Description: strPtr("Tiền thưởng hiệu suất, thưởng tháng 13")},
		{Name: "Đầu tư", Type: CategoryTypeIncome, Description: strPtr("Lợi nhuận từ cổ phiếu, tiền gửi tiết kiệm")},
		{Name: "Khác", Type: CategoryTypeIncome, Description: strPtr("Các nguồn thu nhập vãng lai khác")},
	}

	for _, cat := range defaultCategories {
		cat.UserID = u.ID
		if err := tx.Create(&cat).Error; err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string {
	return &s
}
