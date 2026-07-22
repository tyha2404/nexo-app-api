# Design Spec: Add INVESTMENT Transaction and Category Type

We will add a new type `INVESTMENT` to both Transaction and Category entities to support tracking investments.

## Changes Proposed

### 1. Database & Models
- **`internal/model/category.go`**:
  - Add constant `CategoryTypeInvestment CategoryType = "INVESTMENT"`
  - Update check constraint: `check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')`
- **`internal/model/transaction.go`**:
  - Add constant `TransactionTypeInvestment TransactionType = "INVESTMENT"`
  - Update check constraint: `check:type IN ('INCOME', 'EXPENSE', 'INVESTMENT')`

### 2. Migration
Create a SQL migration file to drop existing check constraints and create new ones that include `INVESTMENT`.
- File: `internal/migration/schema/20260719000000_add_investment_type.sql`
- Content:
  ```sql
  ALTER TABLE categories DROP CONSTRAINT IF EXISTS chk_categories_type;
  ALTER TABLE categories ADD CONSTRAINT chk_categories_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));

  ALTER TABLE transactions DROP CONSTRAINT IF EXISTS chk_transactions_type;
  ALTER TABLE transactions ADD CONSTRAINT chk_transactions_type CHECK (type IN ('INCOME', 'EXPENSE', 'INVESTMENT'));
  ```

### 3. DTO & Validation
- **`internal/dto/category_dto.go`**:
  - Update validation in `CreateCategoryRequest` and `UpdateCategoryRequest` to `oneof=INCOME EXPENSE INVESTMENT`.
- **`internal/dto/transaction_dto.go`**:
  - Update validation in `CreateTransactionRequest` and `UpdateTransactionRequest` to `oneof=INCOME EXPENSE INVESTMENT`.

### 4. Service Logic & Reports
- **`internal/dto/report.go`**:
  - Add `TotalInvestment` to `SummaryReport` response structure.
- **`internal/service/report_service.go`**:
  - Calculate `TotalInvestment` in `GetSummary` by summing up all transactions with type `INVESTMENT` in the specified time frame.
  - Compute `NetBalance` as `TotalIncome - TotalExpense` (Option A: investment is not subtracted from net balance).
