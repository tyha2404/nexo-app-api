# Add INVESTMENT Transaction and Category Type Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new transaction and category type `INVESTMENT` to track investments and show total investments in the summary report.

**Architecture:** Update constants, GORM check constraints, and validators in `model` and `dto` layers. Write a migration script to update DB check constraints. Update `reportService` to compute and return total investment in the summary.

**Tech Stack:** Go (Golang), GORM, PostgreSQL, Goose Migrator, Chi Router.

---

### Task 1: Create and Run SQL Migration

**Files:**
- Create: `internal/migration/schema/20260719000000_add_investment_type.sql`

- [ ] **Step 1: Write the goose migration file**

```sql
-- +goose Up
-- Drop old check constraints if they exist
ALTER TABLE categories DROP CONSTRAINT IF EXISTS chk_categories_type;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_type;

-- Add updated check constraints including 'INVESTMENT'
ALTER TABLE categories ADD CONSTRAINT chk_categories_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));
ALTER TABLE transactions ADD CONSTRAINT chk_transactions_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));

-- +goose Down
-- Revert constraints to only allow 'INCOME' and 'EXPENSE'
ALTER TABLE categories DROP CONSTRAINT IF EXISTS chk_categories_type;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_type;

ALTER TABLE categories ADD CONSTRAINT chk_categories_type CHECK (type IN ('INCOME', 'EXPENSE'));
ALTER TABLE transactions ADD CONSTRAINT chk_transactions_type CHECK (type IN ('INCOME', 'EXPENSE'));
```

- [ ] **Step 2: Run goose up to verify the migration succeeds**

Run: `go run cmd/migrate/main.go up`
Expected: Migration executes successfully.

---

### Task 2: Update Models

**Files:**
- Modify: `internal/model/category.go`
- Modify: `internal/model/transaction.go`

- [ ] **Step 1: Update Category model**

Modify: `internal/model/category.go`
```go
// Add CategoryTypeInvestment constant under CategoryType const block (around line 12-15):
CategoryTypeInvestment CategoryType = "INVESTMENT"

// Update Category struct's Type field tags (around line 21):
Type        CategoryType `gorm:"type:varchar(10);not null;default:'EXPENSE';check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')" json:"type"`
```

- [ ] **Step 2: Update Transaction model**

Modify: `internal/model/transaction.go`
```go
// Add TransactionTypeInvestment constant under TransactionType const block (around line 12-15):
TransactionTypeInvestment TransactionType = "INVESTMENT"

// Update Transaction struct's Type field tags (around line 22):
Type            TransactionType `gorm:"type:varchar(10);not null;check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')" json:"type"`
```

---

### Task 3: Update DTOs & Validation

**Files:**
- Modify: `internal/dto/category_dto.go`
- Modify: `internal/dto/transaction_dto.go`

- [ ] **Step 1: Update Category DTO validation**

Modify: `internal/dto/category_dto.go`
```go
// In CreateCategoryRequest (around line 5):
Type        string  `json:"type" example:"EXPENSE" validate:"required,oneof=INCOME EXPENSE INVESTMENT"`

// In UpdateCategoryRequest (around line 11):
Type        *string `json:"type,omitempty" example:"EXPENSE" validate:"omitempty,oneof=INCOME EXPENSE INVESTMENT"`
```

- [ ] **Step 2: Update Transaction DTO validation**

Modify: `internal/dto/transaction_dto.go`
```go
// In CreateTransactionRequest (around line 12):
Type            string    `json:"type" example:"EXPENSE" validate:"required,oneof=INCOME EXPENSE INVESTMENT"`

// In UpdateTransactionRequest (around line 20):
Type            *string    `json:"type,omitempty" example:"INCOME" validate:"omitempty,oneof=INCOME EXPENSE INVESTMENT"`
```

---

### Task 4: Update Reports

**Files:**
- Modify: `internal/dto/report.go`
- Modify: `internal/service/report_service.go`

- [ ] **Step 1: Update SummaryReport DTO**

Modify: `internal/dto/report.go`
```go
type SummaryReport struct {
	TotalIncome      float64 `json:"totalIncome"`
	TotalExpense     float64 `json:"totalExpense"`
	TotalInvestment  float64 `json:"totalInvestment"` // Added
	NetBalance       float64 `json:"netBalance"`
}
```

- [ ] **Step 2: Update report service calculation**

Modify: `internal/service/report_service.go` inside `GetSummary` (around lines 27-65):
```go
func (s *reportService) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate string) (*dto.SummaryReport, error) {
	// 1. Fetch Income transactions
	incomeFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeIncome),
		"startDate": startDate,
		"endDate":   endDate,
	}
	incomes, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, incomeFilters)
	if err != nil {
		return nil, err
	}

	var totalIncome float64
	for _, inc := range incomes {
		totalIncome += inc.Amount
	}

	// 2. Fetch Expense transactions
	expenseFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeExpense),
		"startDate": startDate,
		"endDate":   endDate,
	}
	expenses, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, expenseFilters)
	if err != nil {
		return nil, err
	}

	var totalExpense float64
	for _, exp := range expenses {
		totalExpense += exp.Amount
	}

	// 3. Fetch Investment transactions
	investmentFilters := map[string]interface{}{
		"type":      string(model.TransactionTypeInvestment),
		"startDate": startDate,
		"endDate":   endDate,
	}
	investments, _, err := s.transactionRepo.ListByUserID(ctx, userID, 10000, 0, investmentFilters)
	if err != nil {
		return nil, err
	}

	var totalInvestment float64
	for _, inv := range investments {
		totalInvestment += inv.Amount
	}

	return &dto.SummaryReport{
		TotalIncome:     totalIncome,
		TotalExpense:    totalExpense,
		TotalInvestment: totalInvestment,
		NetBalance:      totalIncome - totalExpense,
	}, nil
}
```

---

### Task 5: Verify & Run Tests

**Files:**
- Test: `internal/service/report_service_test.go` or execute `go test ./...`

- [ ] **Step 1: Run all tests to make sure they pass**

Run: `go test ./...`
Expected: PASS
