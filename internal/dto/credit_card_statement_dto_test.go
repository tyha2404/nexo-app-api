package dto

import (
	"testing"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestStatementDueTodayIsNotOverdue(t *testing.T) {
	now := util.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC) // DATE column value
	s := &model.CreditCardStatement{StatementBalance: 100, DueDate: today, Status: model.StatementStatusUnpaid}
	if got := ToStatementResponse(s).Status; got != model.StatementStatusUnpaid {
		t.Fatalf("due today: got %s", got)
	}
	s.DueDate = today.AddDate(0, 0, -1)
	if got := ToStatementResponse(s).Status; got != model.StatementStatusOverdue {
		t.Fatalf("due yesterday: got %s", got)
	}
}
