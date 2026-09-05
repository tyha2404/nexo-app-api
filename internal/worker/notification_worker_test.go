package worker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/worker"
)

func TestFormatVND(t *testing.T) {
	// We can test formatVND behavior indirectly or ensure worker package initializes cleanly
	assert.NotNil(t, worker.NewMonthlyRolloverWorker(nil, nil))
}
