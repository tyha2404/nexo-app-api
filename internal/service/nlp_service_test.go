package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNLPService_ParseTransaction(t *testing.T) {
	service := NewNLPService(nil)
	ctx := context.Background()
	userID := uuid.New()

	tests := []struct {
		name                 string
		input                string
		expectedAmount       float64
		expectedType         string
		expectedDesc         string
		expectedCategoryName string
	}{
		{
			name:                 "Ăn phở 65k",
			input:                "Ăn phở 65k",
			expectedAmount:       65000,
			expectedType:         "EXPENSE",
			expectedDesc:         "Ăn phở",
			expectedCategoryName: "Ăn uống",
		},
		{
			name:                 "Đổ xăng 50.000d",
			input:                "Đổ xăng 50.000d",
			expectedAmount:       50000,
			expectedType:         "EXPENSE",
			expectedDesc:         "Đổ xăng",
			expectedCategoryName: "Di chuyển",
		},
		{
			name:                 "Lương tháng này 15tr",
			input:                "Lương tháng này 15tr",
			expectedAmount:       15000000,
			expectedType:         "INCOME",
			expectedDesc:         "Lương tháng này",
			expectedCategoryName: "Lương",
		},
		{
			name:                 "Cà phê 35 nghìn",
			input:                "Cà phê 35 nghìn",
			expectedAmount:       35000,
			expectedType:         "EXPENSE",
			expectedDesc:         "Cà phê",
			expectedCategoryName: "Ăn uống",
		},
		{
			name:                 "1,5 củ mua đồ",
			input:                "1,5 củ mua đồ",
			expectedAmount:       1500000,
			expectedType:         "EXPENSE",
			expectedDesc:         "mua đồ",
			expectedCategoryName: "",
		},
		{
			name:                 "Thu nhập 5 triệu",
			input:                "Thu nhập 5 triệu",
			expectedAmount:       5000000,
			expectedType:         "INCOME",
			expectedDesc:         "Thu nhập",
			expectedCategoryName: "Lương",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.ParseTransaction(ctx, userID, tt.input)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedAmount, resp.Amount)
			assert.Equal(t, tt.expectedType, resp.Type)
			assert.Equal(t, tt.expectedDesc, resp.Description)
			if tt.expectedCategoryName != "" {
				assert.Equal(t, tt.expectedCategoryName, resp.CategoryName)
			}
		})
	}
}
