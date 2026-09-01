# Design Specification: Monthly Target and Budget Auto-Setup Worker

**Date**: 2026-09-01  
**Status**: Approved  
**Target Module**: `nexo-app-api`

---

## 1. Overview & Objectives

In the Nexo financial management system, users set:
1. **Monthly Targets** (`monthly_targets` table): High-level monthly targets for `EXPENSE` (spending cap) and `INVESTMENT` (investment target).
2. **Category Budgets** (`budgets` table): Specific monthly budget amounts per category (`period_type = 'monthly'`).

This feature introduces a background worker service in `nexo-app-api` that automatically rolls over / sets up the monthly targets and category budgets for all active users prior to the start of each new month (or upon server startup if backfill is required).

---

## 2. Requirements & Business Rules

### 2.1 Scheduling & Timing
- **Cron Schedule**: Runs daily at **23:50** (`50 23 * * *`).
- **Trigger Condition**: If adding 15 minutes to the current timestamp crosses into a new month (`now.Add(15*time.Minute).Month() != now.Month()`), the worker triggers a rollover job for the upcoming month (`targetMonth`, `targetYear`).
- **Startup Backfill**: When the API server starts, the worker asynchronously performs a backfill check for the **current month** (`now.Month()`, `now.Year()`) to ensure no active users missed setup during any server downtime.

### 2.2 Target Rollover Logic (`MonthlyTarget`)
For every user in the database and for both target types (`EXPENSE` and `INVESTMENT`):
1. Check if the user already has a target for (`targetMonth`, `targetYear`, `targetType`).
2. If **already exists**: Do nothing (preserve user-configured targets).
3. If **missing**: Find the most recent target of the same `targetType` prior to the target month (`year < targetYear OR (year = targetYear AND month < targetMonth)` ordered by `year DESC, month DESC`).
4. If a previous target is found: Insert a new `MonthlyTarget` record for (`targetMonth`, `targetYear`, `targetType`) with `target_amount = latest.target_amount`.

### 2.3 Budget Rollover Logic (`Budget`)
For every user:
1. Identify all categories where the user had a previous monthly budget (`period_type = 'monthly'`).
2. For each category:
   - Target period start is `Date(targetYear, targetMonth, 1)`.
   - Check if a budget already exists with `user_id = user.ID`, `category_id = category.ID`, `period_type = 'monthly'`, and `period_start = targetPeriodStart`.
   - If **already exists**: Do nothing.
   - If **missing**: Find the most recent monthly budget for that category with `period_start < targetPeriodStart` ordered by `period_start DESC`.
   - If found: Create a new `Budget` record with `amount = latest.amount`, `period_type = 'monthly'`, `period_start = targetPeriodStart`.

### 2.4 Idempotency & Safety
- Database operations use `ON CONFLICT DO NOTHING` (or explicit existence verification) to prevent duplicate key constraint violations (`idx_user_target_month_type`).
- Operations per user are wrapped in transactions or safe batch inserts.
- Failure for one user does not abort the entire job for other users; errors are logged with details.

---

## 3. Architecture & Components

```
+------------------------------------------------------------------+
|                       cmd/server/main.go                         |
|                                                                  |
|  +--------------------+         +-----------------------------+  |
|  |    HTTP Server     |         |    MonthlyRolloverWorker    |  |
|  |  (Chi Router API)  |         |     (robfig/cron/v3)        |  |
|  +--------------------+         +-----------------------------+  |
|                                                |                 |
|                                                v                 |
|                                 +-----------------------------+  |
|                                 |       RolloverService       |  |
|                                 +-----------------------------+  |
|                                                |                 |
|                                                v                 |
|                                 +-----------------------------+  |
|                                 |     RolloverRepository      |  |
|                                 +-----------------------------+  |
+------------------------------------------------------------------+
```

### 3.1 New & Modified Packages
1. **Dependencies (`go.mod`)**:
   - `github.com/robfig/cron/v3`
2. **Repository (`internal/repository/rollover_repo.go`)**:
   - `GetAllUserIDs(ctx context.Context) ([]uuid.UUID, error)`
   - `GetLatestTargetBefore(ctx context.Context, userID uuid.UUID, targetType model.TargetType, month, year int) (*model.MonthlyTarget, error)`
   - `GetLatestBudgetsBefore(ctx context.Context, userID uuid.UUID, periodStart time.Time) ([]model.Budget, error)`
   - `CreateMonthlyTargetsBatch(ctx context.Context, targets []model.MonthlyTarget) error`
   - `CreateBudgetsBatch(ctx context.Context, budgets []model.Budget) error`
3. **Service (`internal/service/rollover_service.go`)**:
   - `ProcessRolloverForMonth(ctx context.Context, month, year int) error`
   - `ProcessRolloverForUser(ctx context.Context, userID uuid.UUID, month, year int) error`
4. **Worker (`internal/worker/monthly_rollover_worker.go`)**:
   - `MonthlyRolloverWorker struct { cron *cron.Cron, service RolloverService, logger *zap.Logger }`
   - `Start()`: Registers cron schedule `50 23 * * *` and launches initial startup sync in background goroutine.
   - `Stop()`: Gracefully stops cron runners.
5. **Server Lifecycle (`cmd/server/main.go`)**:
   - Instantiate worker and call `worker.Start()`.
   - On shutdown signal, call `worker.Stop()`.

---

## 4. Error Handling & Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| User has no previous targets/budgets | Skipped cleanly without creating blank or zeroed targets. |
| User already manually created target for next month | Preserved as-is; not overwritten. |
| Leap year / Month with 28/29/30/31 days | `now.Add(15 * time.Minute).Month() != now.Month()` correctly detects end of month regardless of calendar length. |
| Server was down at 23:50 on the last day | Startup sync automatically catches up and populates the current month. |
| User has budget from 3 months ago but skipped last month | Worker finds the latest historical budget and copies it forward. |
| Concurrent execution or duplicate triggers | `ON CONFLICT DO NOTHING` guarantees idempotency. |

---

## 5. Testing Plan

1. **Unit Tests (`internal/service/rollover_service_test.go`)**:
   - Test rollover when user has previous targets (both EXPENSE and INVESTMENT).
   - Test rollover when user has previous category budgets.
   - Test skipping when target/budget for the target month already exists.
   - Test skipping when user has no historical data.
   - Test handling multiple users with partial existing data.
2. **Worker Scheduling Tests (`internal/worker/monthly_rollover_worker_test.go`)**:
   - Test calculation helper for detecting end-of-month at 23:50.
