# Monthly Target and Budget Auto-Setup Worker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a background worker in `nexo-app-api` that automatically sets up and rolls over `MonthlyTarget` (EXPENSE & INVESTMENT) and monthly `Budget` (per category) for all active users at 23:50 before the start of each month and on server startup.

**Architecture:** A `MonthlyRolloverWorker` using `robfig/cron/v3` runs at `50 23 * * *`, checks if `now + 15m` transitions into a new month, and calls `RolloverService.ProcessRolloverForMonth`. On startup, it triggers backfill for the current month. Repositories handle database retrieval of historical latest targets/budgets and idempotent batch upserts.

**Tech Stack:** Go 1.25, GORM, PostgreSQL, `github.com/robfig/cron/v3`, Uber Zap Logger, `testify`.

---

### Task 1: Add `robfig/cron/v3` Dependency

**Files:**
- Modify: `nexo-app-api/go.mod`
- Modify: `nexo-app-api/go.sum`

- [ ] **Step 1: Add cron v3 to go.mod**

Run command `go get github.com/robfig/cron/v3@v3.0.1` inside `nexo-app-api/`.

- [ ] **Step 2: Verify dependency in go.mod**

Run: `go list -m github.com/robfig/cron/v3`
Expected: `github.com/robfig/cron/v3 v3.0.1`

---

### Task 2: Rollover Repository Interface & Implementation

**Files:**
- Create: `nexo-app-api/internal/repository/rollover_repo.go`

- [ ] **Step 1: Write `RolloverRepository` interface and implementation**

Implement:
- `GetAllActiveUserIDs(ctx context.Context) ([]uuid.UUID, error)`: Select distinct `id` from `users` table where `deleted_at IS NULL`.
- `GetTargetForMonth(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)`: Check if target exists for target month.
- `GetLatestTargetBefore(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)`: Find latest target where `(year < targetYear OR (year = targetYear AND month < targetMonth))`.
- `GetMonthlyBudgetsForPeriod(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error)`: List user's monthly budgets for the specific period start date.
- `GetLatestMonthlyBudgetsBefore(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error)`: Find the most recent monthly budget for each distinct category configured in the past.
- `CreateMonthlyTargets(ctx context.Context, targets []model.MonthlyTarget) error`: Insert targets with `clause.OnConflict{DoNothing: true}`.
- `CreateBudgets(ctx context.Context, budgets []model.Budget) error`: Insert budgets with `clause.OnConflict{DoNothing: true}`.

- [ ] **Step 2: Verify build**

Run: `go build ./internal/repository/...`
Expected: Build passes without error.

---

### Task 3: Rollover Service & Unit Tests

**Files:**
- Create: `nexo-app-api/internal/service/rollover_service.go`
- Create: `nexo-app-api/internal/service/rollover_service_test.go`

- [ ] **Step 1: Write unit tests in `rollover_service_test.go`**

Include test cases:
1. `TestRolloverService_ProcessRolloverForUser_CopiesBothTargetsAndBudgets`: User has targets and budgets in previous month, target month has none -> creates new records.
2. `TestRolloverService_ProcessRolloverForUser_SkipsExisting`: User already has targets for the new month -> does not duplicate.
3. `TestRolloverService_ProcessRolloverForUser_HistoricalJump`: User has targets from 2 months ago (month T-2) -> properly finds and copies to target month.
4. `TestRolloverService_ProcessRolloverForUser_NoHistory`: User has no prior targets or budgets -> skips safely without error.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -v ./internal/service/ -run TestRolloverService`
Expected: FAIL (service not implemented yet)

- [ ] **Step 3: Implement `RolloverService` in `rollover_service.go`**

Implement:
- `ProcessRolloverForMonth(ctx context.Context, targetMonth, targetYear int) error`
- `ProcessRolloverForUser(ctx context.Context, userID uuid.UUID, targetMonth, targetYear int) error`

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -v ./internal/service/ -run TestRolloverService`
Expected: PASS

---

### Task 4: Rollover Background Worker & Lifecycle Management

**Files:**
- Create: `nexo-app-api/internal/worker/monthly_rollover_worker.go`
- Create: `nexo-app-api/internal/worker/monthly_rollover_worker_test.go`

- [ ] **Step 1: Write helper and worker unit tests in `monthly_rollover_worker_test.go`**

Test logic:
- `IsEndOfMonthTransition(t time.Time) (bool, int, int)`: When passed `2026-08-31 23:50:00`, returns `true, 9, 2026`. When passed `2026-08-15 23:50:00`, returns `false, 0, 0`. When passed `2026-12-31 23:50:00`, returns `true, 1, 2027`.

- [ ] **Step 2: Implement `MonthlyRolloverWorker` in `monthly_rollover_worker.go`**

Implement:
- `NewMonthlyRolloverWorker(service service.RolloverService, logger *zap.Logger) *MonthlyRolloverWorker`
- `Start(ctx context.Context)`:
  1. Spawns startup sync goroutine to backfill current month (`now.Month(), now.Year()`).
  2. Sets up cron schedule `50 23 * * *` using standard UTC/Local location.
  3. Checks `IsEndOfMonthTransition(now)` when cron fires and calls `service.ProcessRolloverForMonth`.
- `Stop()`: Calls `cron.Stop()`.

- [ ] **Step 3: Run worker tests**

Run: `go test -v ./internal/worker/...`
Expected: PASS

---

### Task 5: Integrate Worker into `cmd/server/main.go`

**Files:**
- Modify: `nexo-app-api/cmd/server/main.go`

- [ ] **Step 1: Initialize Rollover Repo, Service, and Worker in `main.go`**

Instantiate:
- `rolloverRepo := repository.NewRolloverRepository(gormDB)`
- `rolloverService := service.NewRolloverService(rolloverRepo, logg)`
- `rolloverWorker := worker.NewMonthlyRolloverWorker(rolloverService, logg)`
- Call `rolloverWorker.Start(context.Background())`
- In shutdown block before DB close: Call `rolloverWorker.Stop()`

- [ ] **Step 2: Test full test suite and build**

Run: `go test ./...`
Run: `go build ./cmd/server`
Expected: PASS

---

### Task 6: End-to-End Verification

- [ ] **Step 1: Run all backend tests**
Run: `go test -v ./...` in `nexo-app-api`
Expected: All tests pass.
