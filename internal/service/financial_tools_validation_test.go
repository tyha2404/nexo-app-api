package service

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

// stubWalletRepo implements only ListByUserID; any other call panics, which
// proves the tool stopped before touching balances.
type stubWalletRepo struct {
	repository.WalletRepository
	wallets []model.Wallet
}

func (s *stubWalletRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	return s.wallets, nil
}

func defaultWallets() []model.Wallet {
	return []model.Wallet{
		{ID: uuid.New(), Name: "Ví Tiền mặt"},
		{ID: uuid.New(), Name: "Tài khoản Ngân hàng"},
		{ID: uuid.New(), Name: "Ví Điện tử"},
		{ID: uuid.New(), Name: "Techcombank"},
		{ID: uuid.New(), Name: "Techcombank Visa"},
	}
}

func toolError(t *testing.T, res *FinancialToolResult) string {
	t.Helper()
	require.NotNil(t, res)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res.ResultJSON), &payload), "tool result must be valid JSON: %s", res.ResultJSON)
	msg, _ := payload["error"].(string)
	return msg
}

func TestNormalizeName(t *testing.T) {
	assert.Equal(t, "vi tien mat", normalizeName("  Ví   Tiền MẶT "))
	assert.Equal(t, "vi dien tu", normalizeName("Ví Điện tử"))
}

func TestMatchWalletByName(t *testing.T) {
	wallets := defaultWallets()

	w, err := matchWalletByName(wallets, "vi tien mat")
	require.NoError(t, err)
	assert.Equal(t, "Ví Tiền mặt", w.Name, "diacritics/case-insensitive exact match")

	w, err = matchWalletByName(wallets, "techcombank")
	require.NoError(t, err)
	assert.Equal(t, "Techcombank", w.Name, "exact match wins over a partial match")

	w, err = matchWalletByName(wallets, "ngân hàng")
	require.NoError(t, err)
	assert.Equal(t, "Tài khoản Ngân hàng", w.Name, "unique partial match")

	_, err = matchWalletByName(wallets, "ví")
	require.Error(t, err, "ambiguous partial match must not pick a wallet")
	assert.Contains(t, err.Error(), "nhiều ví")

	_, err = matchWalletByName(wallets, "MoMo")
	require.Error(t, err, "no match must not fall back to the first wallet")

	_, err = matchWalletByName(wallets, "")
	require.Error(t, err, "empty name with several wallets must ask the user")

	w, err = matchWalletByName(wallets[:1], "")
	require.NoError(t, err)
	assert.Equal(t, wallets[0].ID, w.ID, "single wallet is unambiguous")
}

func TestToolNumberArgs(t *testing.T) {
	v, ok := toolPositiveAmount(map[string]interface{}{"amount": 50000.0}, "amount")
	assert.True(t, ok)
	assert.Equal(t, 50000.0, v)

	v, ok = toolPositiveAmount(map[string]interface{}{"amount": "35000"}, "amount")
	assert.True(t, ok)
	assert.Equal(t, 35000.0, v)

	for _, bad := range []interface{}{-1.0, 0.0, math.NaN(), math.Inf(1), "abc", "NaN", true, nil} {
		_, ok = toolPositiveAmount(map[string]interface{}{"amount": bad}, "amount")
		assert.False(t, ok, "amount %v must be rejected", bad)
	}

	fee, ok := toolNonNegativeAmount(map[string]interface{}{}, "fee")
	assert.True(t, ok)
	assert.Equal(t, 0.0, fee)
	_, ok = toolNonNegativeAmount(map[string]interface{}{"fee": -5000.0}, "fee")
	assert.False(t, ok)
	_, ok = toolNonNegativeAmount(map[string]interface{}{"fee": math.NaN()}, "fee")
	assert.False(t, ok)
}

func newToolTestService(wallets []model.Wallet) *chatService {
	return &chatService{walletRepo: &stubWalletRepo{wallets: wallets}, logger: zap.NewNop()}
}

func TestToolCreateTransaction_RejectsBadAmounts(t *testing.T) {
	s := newToolTestService(defaultWallets())
	for _, amount := range []interface{}{-50000.0, 0.0, math.NaN(), "-1"} {
		res, err := s.toolCreateTransaction(context.Background(), uuid.New(), map[string]interface{}{"amount": amount, "type": "EXPENSE"})
		require.NoError(t, err)
		assert.NotEmpty(t, toolError(t, res), "amount %v", amount)
	}
}

func TestToolCreateTransaction_AmbiguousOrMissingWalletAsksUser(t *testing.T) {
	s := newToolTestService(defaultWallets())

	// No wallet named: must not default to the first wallet.
	res, err := s.toolCreateTransaction(context.Background(), uuid.New(), map[string]interface{}{"amount": 50000.0, "type": "EXPENSE"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, res), "Ví Tiền mặt")

	// Ambiguous: "techcom" matches two wallets partially.
	res, err = s.toolCreateTransaction(context.Background(), uuid.New(), map[string]interface{}{"amount": 50000.0, "type": "EXPENSE", "walletName": "techcom"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, res), "nhiều ví")

	// Unknown wallet.
	res, err = s.toolCreateTransaction(context.Background(), uuid.New(), map[string]interface{}{"amount": 50000.0, "type": "EXPENSE", "walletName": "MoMo"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, res), "MoMo")
}

func TestToolTransfer_Validation(t *testing.T) {
	s := newToolTestService(defaultWallets())
	ctx := context.Background()

	res, err := s.toolTransferBetweenWallets(ctx, uuid.New(), map[string]interface{}{"amount": 100000.0, "fee": -1000.0, "fromWalletName": "Techcombank", "toWalletName": "Ví Tiền mặt"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, res), "fee")

	res, err = s.toolTransferBetweenWallets(ctx, uuid.New(), map[string]interface{}{"amount": -100000.0, "fromWalletName": "Techcombank", "toWalletName": "Ví Tiền mặt"})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, res))

	res, err = s.toolTransferBetweenWallets(ctx, uuid.New(), map[string]interface{}{"amount": 100000.0, "fromWalletName": "ví", "toWalletName": "Techcombank"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(toolError(t, res), "Ví nguồn"))

	res, err = s.toolTransferBetweenWallets(ctx, uuid.New(), map[string]interface{}{"amount": 100000.0, "fromWalletName": "Techcombank", "toWalletName": "techcombank"})
	require.NoError(t, err)
	assert.Contains(t, toolError(t, res), "khác nhau")
}

func TestOtherWriteTools_RejectNegativeAmounts(t *testing.T) {
	s := newToolTestService(nil)
	ctx := context.Background()
	uid := uuid.New()

	res, err := s.toolCreateDebt(ctx, uid, map[string]interface{}{"title": "Nợ A", "totalAmount": -5.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, res))

	res, err = s.toolRecordDebtRepayment(ctx, uid, map[string]interface{}{"debtTitle": "Nợ A", "amount": -5.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, res))

	res, err = s.toolSetBudget(ctx, uid, map[string]interface{}{"categoryName": "Ăn uống", "amount": -5.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, res))

	res, err = s.toolSetFinancialTarget(ctx, uid, map[string]interface{}{"targetType": "EXPENSE", "targetAmount": -5.0})
	require.NoError(t, err)
	assert.NotEmpty(t, toolError(t, res))
}

func TestBuildUpstreamMessages_CapsHistory(t *testing.T) {
	placeholder := uuid.New()
	long := strings.Repeat("a", chatHistoryMaxMessageRunes+500)

	var history []model.ChatMessage
	for i := 0; i < chatHistoryMaxMessages+4; i++ {
		history = append(history, model.ChatMessage{ID: uuid.New(), Role: model.ChatRoleUser, Content: long})
	}
	history = append(history, model.ChatMessage{ID: placeholder, Role: model.ChatRoleModel, Content: ""})

	msgs := buildUpstreamMessages("system", history, placeholder)
	require.Equal(t, "system", msgs[0].Role)

	total := 0
	for _, m := range msgs[1:] {
		n := len([]rune(m.Content))
		assert.LessOrEqual(t, n, chatHistoryMaxMessageRunes+1)
		total += n
	}
	assert.LessOrEqual(t, total, chatHistoryMaxTotalRunes+chatHistoryMaxMessageRunes)
	assert.LessOrEqual(t, len(msgs)-1, chatHistoryMaxMessages)

	// Newest messages are kept and order is chronological.
	short := []model.ChatMessage{
		{ID: uuid.New(), Role: model.ChatRoleUser, Content: "first"},
		{ID: uuid.New(), Role: model.ChatRoleModel, Content: "second"},
		{ID: uuid.New(), Role: model.ChatRoleUser, Content: "third"},
	}
	msgs = buildUpstreamMessages("sys", short, placeholder)
	require.Len(t, msgs, 4)
	assert.Equal(t, "first", msgs[1].Content)
	assert.Equal(t, "assistant", msgs[2].Role)
	assert.Equal(t, "third", msgs[3].Content)
}
