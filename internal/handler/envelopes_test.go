package handler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func jsonKeys(v interface{}) []string {
	t := reflect.TypeOf(v)
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		keys = append(keys, strings.Split(t.Field(i).Tag.Get("json"), ",")[0])
	}
	return keys
}

// The Swagger envelope structs must describe the JSON the writers emit.
func TestSwaggerEnvelopes_MatchWriterKeys(t *testing.T) {
	data := []string{"status", "success", "message", "data"}
	page := []string{"status", "success", "message", "items", "total", "page", "limit"}
	for _, v := range []interface{}{
		MessageResponse{}, LoginEnvelope{}, TokenEnvelope{}, LogoutEnvelope{}, ChangePasswordEnvelope{}, UserEnvelope{},
		CategoryEnvelope{}, TransactionEnvelope{}, WalletEnvelope{}, WalletSummaryEnvelope{}, WalletTransferEnvelope{},
		AutoAllocateEnvelope{}, BudgetEnvelope{}, PresetEnvelope{}, PresetListEnvelope{}, TargetSummaryEnvelope{},
		DebtListEnvelope{}, DebtSummaryEnvelope{}, DebtEnvelopeResponse{}, StatementEnvelope{}, StatementListEnvelope{},
		SummaryReportEnvelope{}, CategoryBreakdownEnvelope{}, MonthlyTrendResponse{}, NetWorthResponse{}, DailyReportResponse{},
		ChatSessionEnvelope{}, ChatSessionListEnvelope{}, ChatModelsEnvelope{}, ChatActionResultEnvelope{}, StringEnvelope{},
		CostEnvelope{},
	} {
		assert.Equal(t, data, jsonKeys(v), "%T", v)
	}
	for _, v := range []interface{}{UserPage{}, CategoryPage{}, BudgetPage{}, AlertPage{}} {
		assert.Equal(t, page, jsonKeys(v), "%T", v)
	}
	assert.Equal(t, append(page, "summary"), jsonKeys(TransactionPage{}))
	assert.Equal(t, jsonKeys(paginatedEnvelope{}), append(page, "summary"), "HandlePaginated's envelope")
	assert.Equal(t, []string{"status", "success", "items", "total", "page", "limit"}, jsonKeys(CostPage{}), "legacy /costs list has no message")
}
