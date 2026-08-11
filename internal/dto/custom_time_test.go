package dto_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/dto"
)

func TestCustomTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonInput string
		expected time.Time
		wantErr  bool
	}{
		{
			name:      "RFC3339 format",
			jsonInput: `"2026-08-11T15:04:05Z"`,
			expected:  time.Date(2026, 8, 11, 15, 4, 5, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "Date format YYYY-MM-DD",
			jsonInput: `"2026-08-11"`,
			expected:  time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "Empty string",
			jsonInput: `""`,
			expected:  time.Time{},
			wantErr:   false,
		},
		{
			name:      "Invalid format",
			jsonInput: `"11/08/2026"`,
			expected:  time.Time{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ct dto.CustomTime
			err := json.Unmarshal([]byte(tt.jsonInput), &ct)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error = %v, got %v", tt.wantErr, err)
			}
			if !tt.wantErr && !ct.Equal(tt.expected) {
				t.Errorf("expected time %v, got %v", tt.expected, ct.Time)
			}
		})
	}
}
