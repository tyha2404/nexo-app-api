package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

func TestGetFinancialToolDefinitions(t *testing.T) {
	tools := service.GetFinancialToolDefinitions()
	assert.NotEmpty(t, tools)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		assert.Equal(t, "function", tool.Type)
		assert.NotEmpty(t, tool.Function.Name)
		assert.NotEmpty(t, tool.Function.Description)
		assert.NotNil(t, tool.Function.Parameters)
		toolNames[tool.Function.Name] = true
	}

	assert.True(t, toolNames["get_financial_overview"])
	assert.True(t, toolNames["list_recent_transactions"])
	assert.True(t, toolNames["create_transaction"])
	assert.True(t, toolNames["get_budget_status"])
	assert.True(t, toolNames["get_debt_summary"])
	assert.True(t, toolNames["list_wallets"])
	assert.True(t, toolNames["get_spending_by_category"])

	// Read-only generic tools
	assert.True(t, toolNames["search_transactions"])
	assert.True(t, toolNames["list_categories"])
	assert.True(t, toolNames["get_monthly_trend"])
	assert.True(t, toolNames["get_wallet_detail"])
	assert.True(t, toolNames["get_debt_detail"])
	assert.True(t, toolNames["get_investment_summary"])
	assert.True(t, toolNames["search_financial_knowledge"])
}
