package dto

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CustomTime struct {
	time.Time
}

const ctLayout = time.RFC3339

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	// Remove quotes if present
	if len(s) > 0 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || s == "null" {
		ct.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(ctLayout, s)
	if err != nil {
		return fmt.Errorf("invalid time format, expected RFC3339: %v", err)
	}
	ct.Time = t
	return nil
}

type CreateCostRequest struct {
	Title      string     `json:"title"`
	Amount     float64    `json:"amount"`
	Currency   string     `json:"currency"`
	IncurredAt CustomTime `json:"incurred_at"`
	CategoryID uuid.UUID  `json:"category_id"`
}
