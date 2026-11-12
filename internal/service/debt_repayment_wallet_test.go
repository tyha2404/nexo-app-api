package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

func proposeRepayment(t *testing.T, e *toolEnv, args map[string]interface{}) (*FinancialToolResult, uuid.UUID) {
	t.Helper()
	res, err := e.svc.proposeWriteTool(context.Background(), e.userID, uuid.New(), uuid.Nil,
		ToolCall{ID: "call", Type: "function", Function: ToolCallFunction{Name: "record_debt_repayment", Arguments: mustJSON(t, args)}})
	require.NoError(t, err)
	require.NotNil(t, res)
	if res.ActionCard == nil {
		return res, uuid.Nil
	}
	return res, uuid.MustParse(res.ActionCard.Data.(map[string]interface{})["actionId"].(string))
}

func TestPreviewDebtRepayment_WalletName(t *testing.T) {
	e := newToolEnv(t)
	ctx := context.Background()
	debt := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 1_000_000}
	e.debt.debts = []dto.DebtResponse{debt}
	cash := e.wallets.wallets[0] // "Ví Tiền mặt"

	p, errRes, err := e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 100_000.0, "walletName": "tiền mặt"})
	require.NoError(t, err)
	require.Nil(t, errRes)
	repay := p.args.(*debtRepaymentArgs)
	require.NotNil(t, repay.WalletID)
	assert.Equal(t, cash.ID, *repay.WalletID)
	assert.Equal(t, debt.ID, repay.DebtID)
	assert.Equal(t, cash.Name, p.data["walletName"], "the card shows the wallet")
	assert.Contains(t, p.description, cash.Name)

	// Ambiguous ("Techcombank" + "Techcombank Visa" both contain "tech") and unknown names are asked back.
	_, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "tech"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "nhiều ví")
	_, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "MoMo"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "Không tìm thấy ví 'MoMo'")

	// No wallet named: the repayment moves no balance (unchanged behaviour).
	p, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Nil(t, p.args.(*debtRepaymentArgs).WalletID)

	e.wallets.listErr = errInternalRepo
	_, _, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "tiền mặt"})
	require.ErrorIs(t, err, errInternalRepo)
}

func TestPreviewDebtRepayment_RejectsOverpayAndCompleted(t *testing.T) {
	e := newToolEnv(t)
	ctx := context.Background()
	e.debt.debts = []dto.DebtResponse{
		{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, PaidAmount: 400_000, Remaining: 600_000},
		{ID: uuid.New(), Title: "Cho Lan vay", Type: model.DebtTypeReceivable, TotalAmount: 300_000, PaidAmount: 300_000, Remaining: 0, Status: model.DebtStatusCompleted},
	}

	_, errRes, err := e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 600_001.0})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "vượt quá số còn lại")

	_, errRes, err = e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Lan", "amount": 1.0})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, errRes), "đã được thanh toán đủ")

	// Paying exactly the remaining amount is allowed.
	p, errRes, err := e.svc.previewDebtRepayment(ctx, e.userID, map[string]interface{}{"debtTitle": "Nam", "amount": 600_000.0})
	require.NoError(t, err)
	require.Nil(t, errRes)
	assert.Equal(t, 600_000.0, p.args.(*debtRepaymentArgs).Amount)
}

func TestConfirmDebtRepayment_SetsWalletID(t *testing.T) {
	e := newToolEnv(t)
	debt := dto.DebtResponse{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 1_000_000}
	e.debt.debts = []dto.DebtResponse{debt}
	bank := e.wallets.wallets[1] // "Tài khoản Ngân hàng"

	_, actionID := proposeRepayment(t, e, map[string]interface{}{"debtTitle": "anh Nam", "amount": 200_000.0, "walletName": "ngân hàng"})
	require.NotEqual(t, uuid.Nil, actionID)
	require.Empty(t, e.debt.repaid)

	result, err := e.svc.ConfirmAction(context.Background(), e.userID, actionID)
	require.NoError(t, err)
	require.Len(t, e.debt.repaid, 1)
	assert.Equal(t, debt.ID, e.debt.repaid[0].debtID)
	require.NotNil(t, e.debt.repaid[0].req.WalletID)
	assert.Equal(t, bank.ID, *e.debt.repaid[0].req.WalletID)
	assert.Contains(t, result.ActionCard.Description, bank.Name)
	requireOnlyUser(t, e.userID, e.wallets.users)
}

func TestConfirmDebtRepayment_WalletDeletedMeanwhile(t *testing.T) {
	e := newToolEnv(t)
	e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 1_000_000}}
	_, actionID := proposeRepayment(t, e, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0, "walletName": "tiền mặt"})

	e.wallets.wallets = e.wallets.wallets[1:] // "Ví Tiền mặt" deleted
	_, err := e.svc.ConfirmAction(context.Background(), e.userID, actionID)
	require.ErrorIs(t, err, ErrPendingActionFailed)
	assert.Contains(t, err.Error(), "không còn tồn tại")
	assert.Empty(t, e.debt.repaid)
}

// Business sentinels raised while executing a confirmed proposal are reported
// as a failed action (422) instead of a system error (500).
func TestConfirmDebtRepayment_BusinessErrorsAreNotSystemErrors(t *testing.T) {
	for _, sentinel := range []error{ErrDebtNotFound, ErrInvalidRepayment, ErrDebtAlreadyCompleted, ErrWalletNotFound} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			e := newToolEnv(t)
			e.debt.debts = []dto.DebtResponse{{ID: uuid.New(), Title: "Vay anh Nam", Type: model.DebtTypePayable, TotalAmount: 1_000_000, Remaining: 1_000_000}}
			_, actionID := proposeRepayment(t, e, map[string]interface{}{"debtTitle": "Nam", "amount": 1.0})

			e.debt.repayErr = sentinel
			_, err := e.svc.ConfirmAction(context.Background(), e.userID, actionID)
			require.ErrorIs(t, err, ErrPendingActionFailed)
			assert.Contains(t, err.Error(), sentinel.Error())
		})
	}
}

func TestBusinessSentinelsWrapGenericKinds(t *testing.T) {
	cases := []struct {
		err  error
		kind error
		msg  string
	}{
		{ErrDebtNotFound, constant.ErrNotFound, "debt not found"},
		{repository.ErrDebtNotFound, constant.ErrNotFound, "debt not found"},
		{ErrInvalidRepayment, constant.ErrInvalidInput, "số tiền trả phải lớn hơn 0 và không vượt dư nợ"},
		{ErrDebtAlreadyCompleted, constant.ErrInvalidInput, ""},
		{ErrInvalidDebtUpdate, constant.ErrInvalidInput, "dữ liệu sửa khoản nợ không hợp lệ"},
		{ErrStatementNotFound, constant.ErrNotFound, "không tìm thấy kỳ sao kê"},
		{ErrInvalidDate, constant.ErrInvalidInput, "định dạng ngày không hợp lệ (cần YYYY-MM-DD)"},
		{ErrStatementAlreadyPaid, constant.ErrInvalidInput, "kỳ sao kê đã được thanh toán đủ"},
		{ErrStatementInvalidPayment, constant.ErrInvalidInput, "số tiền thanh toán phải lớn hơn 0 và không vượt số dư còn lại của sao kê"},
		{ErrWalletNotFound, constant.ErrNotFound, "wallet not found"},
	}
	for _, tc := range cases {
		assert.ErrorIs(t, tc.err, tc.kind, tc.err.Error())
		assert.True(t, isBusinessError(tc.err), tc.err.Error())
		if tc.msg != "" {
			assert.Equal(t, tc.msg, tc.err.Error(), "the message is unchanged")
		}
		wrapped := errors.Join(errors.New("ctx"), tc.err)
		assert.ErrorIs(t, wrapped, tc.err, "the sentinel itself still matches")
	}
	assert.NotErrorIs(t, ErrDebtNotFound, constant.ErrInvalidInput)
	assert.NotErrorIs(t, ErrInvalidRepayment, constant.ErrNotFound)
}
