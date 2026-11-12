package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func TestSummarizeWalletsSeparatesCurrencies(t *testing.T) {
	s := SummarizeWallets([]model.Wallet{
		{Balance: 1000000, Currency: "VND", IsIncludedInTotal: true},
		{Balance: 500000, Currency: "", IsIncludedInTotal: true}, // legacy default VND
		{Balance: 200, Currency: "USD", IsIncludedInTotal: true},
		{Balance: 9999999, Currency: "VND", IsIncludedInTotal: false},
	})
	require.Equal(t, 1500000.0, s.TotalBalance)
	require.Equal(t, "VND", s.BaseCurrency)
	require.Equal(t, map[string]float64{"VND": 1500000, "USD": 200}, s.TotalsByCurrency)
	require.Equal(t, 4, s.TotalWallets)
	require.Len(t, s.Wallets, 4)
}
