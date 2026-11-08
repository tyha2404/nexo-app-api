package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
)

// validatorTestPayload exercises every tag getErrorMessage knows about.
type validatorTestPayload struct {
	Required string  `validate:"required"`
	Min      string  `validate:"omitempty,min=3"`
	Max      string  `validate:"omitempty,max=2"`
	Email    string  `validate:"omitempty,email"`
	Len      string  `validate:"omitempty,len=4"`
	Alnum    string  `validate:"omitempty,alphanum"`
	Username string  `validate:"omitempty,username"`
	Gt       float64 `validate:"omitempty,gt=10"`
	Gte      float64 `validate:"omitempty,gte=10"`
	Lt       float64 `validate:"omitempty,lt=10"`
	Lte      float64 `validate:"omitempty,lte=10"`
	OneOf    string  `validate:"omitempty,oneof=A B"`
	URL      string  `validate:"omitempty,url"`
}

func TestValidator_ValidateStruct_Messages(t *testing.T) {
	ok := validatorTestPayload{Required: "x"}
	tests := []struct {
		name    string
		mutate  func(p *validatorTestPayload)
		wantMsg string
	}{
		{"required", func(p *validatorTestPayload) { p.Required = "" }, "Required is required"},
		{"min", func(p *validatorTestPayload) { p.Min = "ab" }, "Min must be at least 3 characters"},
		{"max", func(p *validatorTestPayload) { p.Max = "abc" }, "Max must be at most 2 characters"},
		{"email", func(p *validatorTestPayload) { p.Email = "not-an-email" }, "Email must be a valid email address"},
		{"len", func(p *validatorTestPayload) { p.Len = "abc" }, "Len must be exactly 4 characters"},
		{"alphanum", func(p *validatorTestPayload) { p.Alnum = "a-b" }, "Alnum must contain only alphanumeric characters"},
		{"username", func(p *validatorTestPayload) { p.Username = "bad name!" }, "Username must contain only letters, numbers, underscores or hyphens"},
		{"gt", func(p *validatorTestPayload) { p.Gt = 5 }, "Gt must be greater than 10"},
		{"gte", func(p *validatorTestPayload) { p.Gte = 5 }, "Gte must be greater than or equal to 10"},
		{"lt", func(p *validatorTestPayload) { p.Lt = 50 }, "Lt must be less than 10"},
		{"lte", func(p *validatorTestPayload) { p.Lte = 50 }, "Lte must be less than or equal to 10"},
		{"oneof", func(p *validatorTestPayload) { p.OneOf = "C" }, "OneOf must be one of: A B"},
		{"default", func(p *validatorTestPayload) { p.URL = "::nope" }, "URL is invalid"},
	}
	v := NewValidator()
	require.NoError(t, v.ValidateStruct(ok))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ok
			tt.mutate(&p)
			err := v.ValidateStruct(p)
			require.Error(t, err)
			assert.Equal(t, "validation failed: "+tt.wantMsg, err.Error())
		})
	}
}

func TestValidator_ValidateStruct_JoinsMultipleErrors(t *testing.T) {
	err := NewValidator().ValidateStruct(validatorTestPayload{Min: "a", OneOf: "Z"})
	require.Error(t, err)
	assert.Equal(t, "validation failed: Required is required; Min must be at least 3 characters; OneOf must be one of: A B", err.Error())
}

func TestValidator_ValidateStruct_NonStructIsInvalidInput(t *testing.T) {
	// validator returns InvalidValidationError (not ValidationErrors) for non-structs.
	err := NewValidator().ValidateStruct("just a string")
	assert.ErrorIs(t, err, constant.ErrInvalidInput)
}

func TestValidator_UsernameRule(t *testing.T) {
	type payload struct {
		Name string `validate:"username"`
	}
	v := NewValidator()
	for _, good := range []string{"tyha", "ty_ha-2404", "A1"} {
		assert.NoError(t, v.ValidateStruct(payload{good}), good)
	}
	for _, bad := range []string{"ty ha", "tý", "a.b", "a@b", ""} {
		assert.Error(t, v.ValidateStruct(payload{bad}), bad)
	}
}

func TestValidator_ValidateRequest(t *testing.T) {
	type payload struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := NewValidator()

	var p payload
	require.NoError(t, v.ValidateRequest(validatorTestReq(`{"email":"a@b.co"}`), &p))
	assert.Equal(t, "a@b.co", p.Email)

	p = payload{}
	err := v.ValidateRequest(validatorTestReq(`{"email":`), &p)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "validation failed", "decode errors are returned as-is")

	p = payload{}
	err = v.ValidateRequest(validatorTestReq(`{"email":"nope"}`), &p)
	require.EqualError(t, err, "validation failed: Email must be a valid email address")
}

func TestValidator_ValidatePartial(t *testing.T) {
	type payload struct {
		Name     *string  `json:"name" validate:"omitempty,min=3"`
		Role     string   `json:"role" validate:"oneof=user admin"`
		Amount   *float64 `json:"amount" validate:"omitempty,gt=0"`
		Untagged string   `json:"untagged"`
	}
	v := NewValidator()

	t.Run("zero fields are skipped, even when their rule would fail", func(t *testing.T) {
		var p payload
		// Role is empty → skipped although oneof would reject "".
		require.NoError(t, v.ValidatePartial(validatorTestReq(`{"untagged":"x"}`), &p))
		assert.Equal(t, "x", p.Untagged)
	})

	t.Run("present valid fields pass", func(t *testing.T) {
		var p payload
		require.NoError(t, v.ValidatePartial(validatorTestReq(`{"name":"abcd","role":"admin","amount":5}`), &p))
		require.NotNil(t, p.Name)
		assert.Equal(t, "abcd", *p.Name)
	})

	t.Run("present invalid fields fail", func(t *testing.T) {
		var p payload
		err := v.ValidatePartial(validatorTestReq(`{"name":"ab","role":"root","amount":-1}`), &p)
		require.Error(t, err)
		msg := err.Error()
		assert.True(t, strings.HasPrefix(msg, "validation failed: "), msg)
		assert.Contains(t, msg, "must be at least 3 characters")
		assert.Contains(t, msg, "must be one of: user admin")
		assert.Contains(t, msg, "must be greater than 0")
	})

	t.Run("decode error", func(t *testing.T) {
		var p payload
		assert.Error(t, v.ValidatePartial(validatorTestReq(`{`), &p))
	})

	t.Run("non-struct destination is accepted", func(t *testing.T) {
		var m map[string]interface{}
		require.NoError(t, v.ValidatePartial(validatorTestReq(`{"a":1}`), &m))
		assert.Equal(t, float64(1), m["a"])
	})
}

func TestValidator_RegisterCustomValidation(t *testing.T) {
	type payload struct {
		Code string `validate:"vn_code"`
	}
	v := NewValidator()
	require.NoError(t, v.RegisterCustomValidation("vn_code", func(fl validator.FieldLevel) bool {
		return strings.HasPrefix(fl.Field().String(), "VN")
	}))
	assert.NoError(t, v.ValidateStruct(payload{"VN01"}))
	assert.EqualError(t, v.ValidateStruct(payload{"US01"}), "validation failed: Code is invalid")
}

func validatorTestReq(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
}

// `required` on struct-typed fields (dates) must reject a missing value.
func TestValidator_RequiredDateFields(t *testing.T) {
	v := NewValidator()
	cat := uuid.New()
	now := dto.CustomTime{Time: time.Now()}

	tests := []struct {
		name    string
		payload any
		wantErr bool
	}{
		{"transaction without date", dto.CreateTransactionRequest{CategoryID: cat, Amount: 1, Type: "EXPENSE"}, true},
		{"transaction with date", dto.CreateTransactionRequest{CategoryID: cat, Amount: 1, Type: "EXPENSE", TransactionDate: now}, false},
		{"cost without date", dto.CreateCostRequest{CategoryID: cat, Title: "Cà phê", Amount: 1, Currency: "VND"}, true},
		{"budget without period start", dto.CreateBudgetRequest{CategoryID: cat, Amount: 1, PeriodType: "monthly"}, true},
		{"budget with period start", dto.CreateBudgetRequest{CategoryID: cat, Amount: 1, PeriodType: "monthly", PeriodStart: time.Now()}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateStruct(tt.payload)
			assert.Equal(t, tt.wantErr, err != nil, "err = %v", err)
		})
	}
}
