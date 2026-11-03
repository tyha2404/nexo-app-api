package dto

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestReceiptURLValidation(t *testing.T) {
	v := validator.New()
	str := func(s string) *string { return &s }
	cases := []struct {
		url *string
		ok  bool
	}{
		{nil, true},
		{str(""), true}, // the web client sends "" to clear the receipt
		{str("https://x.supabase.co/storage/v1/object/public/receipts/u/a.jpg"), true},
		{str("javascript:alert(document.cookie)"), false},
		{str("data:text/html,<script>alert(1)</script>"), false},
	}
	for _, tc := range cases {
		err := v.Struct(UpdateTransactionRequest{ReceiptURL: tc.url})
		if (err == nil) != tc.ok {
			desc := "<nil>"
			if tc.url != nil {
				desc = *tc.url
			}
			t.Errorf("receiptUrl %q: expected ok=%v, got err=%v", desc, tc.ok, err)
		}
	}
}
