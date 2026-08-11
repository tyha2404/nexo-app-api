package dto

import (
	"fmt"
	"strings"
	"time"
)

type CustomTime struct {
	time.Time
}

const (
	ctLayoutRFC3339 = time.RFC3339
	ctLayoutDate    = "2006-01-02"
)

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		ct.Time = time.Time{}
		return nil
	}

	// Try RFC3339 format first (e.g. 2026-08-11T00:00:00Z)
	if t, err := time.Parse(ctLayoutRFC3339, s); err == nil {
		ct.Time = t
		return nil
	}

	// Try date format (e.g. 2026-08-11)
	if t, err := time.Parse(ctLayoutDate, s); err == nil {
		ct.Time = t
		return nil
	}

	return fmt.Errorf("invalid time format %q, expected RFC3339 or YYYY-MM-DD", s)
}
