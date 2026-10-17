package model

import "testing"

func TestTransactionWalletBalanceEffect(t *testing.T) {
	st := func(s InvestmentStatus) *InvestmentStatus { return &s }
	pnl := func(v float64) *float64 { return &v }
	cases := []struct {
		name string
		tx   Transaction
		want float64
	}{
		{"income adds", Transaction{Type: TransactionTypeIncome, Amount: 100}, 100},
		{"expense subtracts", Transaction{Type: TransactionTypeExpense, Amount: 100}, -100},
		{"investment holding subtracts", Transaction{Type: TransactionTypeInvestment, Amount: 100, Status: st(InvestmentStatusHolding)}, -100},
		{"investment legacy nil status subtracts", Transaction{Type: TransactionTypeInvestment, Amount: 100}, -100},
		{"investment sold nets realized pnl", Transaction{Type: TransactionTypeInvestment, Amount: 100, Status: st(InvestmentStatusSold), RealizedPnL: pnl(25)}, 25},
		{"investment matured with loss", Transaction{Type: TransactionTypeInvestment, Amount: 100, Status: st(InvestmentStatusMatured), RealizedPnL: pnl(-40)}, -40},
		{"investment cancelled returns principal", Transaction{Type: TransactionTypeInvestment, Amount: 100, Status: st(InvestmentStatusCancelled)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tx.WalletBalanceEffect(); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestDebtWalletDeltas(t *testing.T) {
	if DebtCreationWalletDelta(DebtTypePayable, 100) != 100 || DebtRepaymentWalletDelta(DebtTypePayable, 30) != -30 {
		t.Fatal("payable: borrowing adds to wallet, repaying subtracts")
	}
	if DebtCreationWalletDelta(DebtTypeReceivable, 100) != -100 || DebtRepaymentWalletDelta(DebtTypeReceivable, 30) != 30 {
		t.Fatal("receivable: lending subtracts from wallet, being repaid adds")
	}
}
