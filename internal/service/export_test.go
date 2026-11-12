package service

import (
	"context"

	"github.com/google/uuid"
)

// ProcessRolloverForUser runs the per-user rollover step of svc (built by
// NewRolloverService) for the external service_test package.
func ProcessRolloverForUser(ctx context.Context, svc RolloverService, userID uuid.UUID, targetMonth, targetYear int) error {
	return svc.(*rolloverService).processRolloverForUser(ctx, userID, targetMonth, targetYear)
}
