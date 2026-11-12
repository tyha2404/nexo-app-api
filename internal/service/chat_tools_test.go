package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func TestToolRegistry_WriteToolsAndPromptList(t *testing.T) {
	want := []string{"create_transaction", "create_category", "transfer_between_wallets", "create_debt", "record_debt_repayment", "set_budget", "set_financial_target"}
	assert.Equal(t, want, chatToolRegistry.writeToolNames())
	// The generated list must keep the prompt byte-identical to the
	// hand-written one it replaced.
	assert.Equal(t, "create_transaction, create_category, transfer_between_wallets, create_debt, record_debt_repayment, set_budget, set_financial_target", writeToolListForPrompt())
	assert.Len(t, chatWriteTools, len(want))
	for _, name := range want {
		assert.True(t, IsChatWriteTool(name), name)
	}
	assert.False(t, IsChatWriteTool("list_wallets"))
	assert.False(t, IsChatWriteTool("unknown"))
}

func TestToolRegistry_PromptMentionsOnlyRegisteredTools(t *testing.T) {
	prompt := chatSystemPrompt("2026-11-12")
	for _, m := range regexp.MustCompile(`"([a-z]+(?:_[a-z]+)+)"`).FindAllStringSubmatch(prompt, -1) {
		if m[1] == "pending_confirmation" {
			continue
		}
		assert.NotNil(t, chatToolRegistry.lookup(m[1]), "prompt names unknown tool %q", m[1])
	}
}

func TestToolRegistry_RejectsInvalidSpecs(t *testing.T) {
	read := chatTool{def: ToolDefinition{Function: ToolFunction{Name: "a"}}, run: (*chatService).toolListWallets}
	assert.Panics(t, func() { newChatToolRegistry([]chatTool{read, read}) }, "duplicate name")
	assert.Panics(t, func() { newChatToolRegistry([]chatTool{{def: read.def}}) }, "neither run nor preview")
	both := read
	both.preview = (*chatService).previewCreateDebt
	assert.Panics(t, func() { newChatToolRegistry([]chatTool{both}) }, "both run and preview")
}

func TestProposeWriteTool_RejectsReadTools(t *testing.T) {
	e := newToolEnv(t)
	_, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.New(), toolCall(t, "c", "list_wallets", nil))
	require.Error(t, err)
}

// proposeAndConfirm proposes a write tool call, runs between, then confirms.
func proposeAndConfirm(t *testing.T, e *toolEnv, tool string, args map[string]interface{}, between func()) error {
	t.Helper()
	res, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.Nil, toolCall(t, "c", tool, args))
	require.NoError(t, err)
	require.NotNil(t, res.ActionCard, "expected a proposal: %s", res.ResultJSON)
	actionID := uuid.MustParse(res.ActionCard.Data.(map[string]interface{})["actionId"].(string))
	if between != nil {
		between()
	}
	_, err = e.svc.ConfirmAction(context.Background(), e.userID, actionID)
	return err
}

func TestConfirm_RechecksWalletsDeletedSincePreview(t *testing.T) {
	e := newToolEnv(t)
	e.addCategory("Ăn uống", model.CategoryTypeExpense)
	err := proposeAndConfirm(t, e, "create_transaction", map[string]interface{}{"amount": 1.0, "walletName": "tiền mặt"},
		func() { e.wallets.wallets = e.wallets.wallets[1:] })
	require.ErrorIs(t, err, ErrPendingActionFailed)
	assert.Contains(t, err.Error(), "Ví đã chọn không còn tồn tại")
	assert.Empty(t, e.txnSvc.created)

	e = newToolEnv(t)
	err = proposeAndConfirm(t, e, "transfer_between_wallets", map[string]interface{}{"amount": 1.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
		func() { e.wallets.wallets = nil })
	require.ErrorIs(t, err, ErrPendingActionFailed, "no wallets left is a business error (422), not a system error")
	assert.Contains(t, err.Error(), "đề xuất chuyển tiền")
	assert.Empty(t, e.wallet.transfers)
}

func TestConfirm_UsesTheWalletsThePreviewResolved(t *testing.T) {
	e := newToolEnv(t)
	from, to := e.wallets.wallets[0].ID, e.wallets.wallets[1].ID
	// Renaming a wallet after the preview does not change (or break) the
	// confirmed transfer: it runs on the previewed wallet ids.
	err := proposeAndConfirm(t, e, "transfer_between_wallets", map[string]interface{}{"amount": 5.0, "fromWalletName": "tiền mặt", "toWalletName": "ngân hàng"},
		func() { e.wallets.wallets[0].Name = "Ví mới" })
	require.NoError(t, err)
	require.Len(t, e.wallet.transfers, 1)
	assert.Equal(t, from, e.wallet.transfers[0].FromWalletID)
	assert.Equal(t, to, e.wallet.transfers[0].ToWalletID)
}

func TestConfirm_ActionWithoutArgsFailsAndStaysPending(t *testing.T) {
	e := newToolEnv(t)
	id := uuid.New()
	e.svc.pendingActions.Save(PendingAction{ID: id, UserID: e.userID, ToolName: "create_debt"})
	_, err := e.svc.ConfirmAction(context.Background(), e.userID, id)
	require.Error(t, err)
	got, getErr := e.svc.pendingActions.Get(e.userID, id)
	require.NoError(t, getErr)
	assert.Equal(t, PendingActionPending, got.Status)
}
