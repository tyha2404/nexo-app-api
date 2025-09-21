package model

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id,omitempty"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username" validate:"required,min=3,max=50"`
	Email     string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email" validate:"required,email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"` // This will store the hashed password in the database
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at,omitempty"`
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
