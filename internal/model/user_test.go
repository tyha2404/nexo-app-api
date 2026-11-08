package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestUserValidate(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{"valid", User{Username: "tyha", Email: "tyha@example.com"}, false},
		{"username too short", User{Username: "ty", Email: "tyha@example.com"}, true},
		{"username too long", User{Username: strings.Repeat("a", 51), Email: "tyha@example.com"}, true},
		{"missing username", User{Email: "tyha@example.com"}, true},
		{"invalid email", User{Username: "tyha", Email: "not-an-email"}, true},
		{"missing email", User{Username: "tyha"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.user.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserHashAndCheckPassword(t *testing.T) {
	u := User{Password: "s3cret-password"}
	if err := u.HashPassword(); err != nil {
		t.Fatal(err)
	}
	if u.Password == "s3cret-password" || !strings.HasPrefix(u.Password, "$2") {
		t.Fatalf("expected a bcrypt hash, got %q", u.Password)
	}
	if err := u.CheckPassword("s3cret-password"); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := u.CheckPassword("wrong"); err == nil {
		t.Error("wrong password accepted")
	}

	// Two hashes of the same password differ (salted).
	other := User{Password: "s3cret-password"}
	_ = other.HashPassword()
	if other.Password == u.Password {
		t.Error("expected different salts")
	}

	// Supabase-synced users have no local hash: nothing matches.
	if err := (&User{}).CheckPassword(""); err == nil {
		t.Error("empty hash must not accept an empty password")
	}
}

func TestBeforeCreateAssignsUUIDv7(t *testing.T) {
	models := map[string]func() idHook{
		"User": func() idHook { return &User{} }, "Category": func() idHook { return &Category{} },
		"ChatSession": func() idHook { return &ChatSession{} }, "ChatMessage": func() idHook { return &ChatMessage{} },
		"FinancialKnowledge": func() idHook { return &FinancialKnowledge{} }, "Budget": func() idHook { return &Budget{} },
		"Alert": func() idHook { return &Alert{} }, "Debt": func() idHook { return &Debt{} },
		"Repayment": func() idHook { return &Repayment{} }, "Expense": func() idHook { return &Expense{} },
		"RefreshToken": func() idHook { return &RefreshToken{} }, "Cost": func() idHook { return &Cost{} },
		"PushSubscription": func() idHook { return &PushSubscription{} }, "Transaction": func() idHook { return &Transaction{} },
		"Preset": func() idHook { return &Preset{} }, "MonthlyTarget": func() idHook { return &MonthlyTarget{} },
		"Wallet": func() idHook { return &Wallet{} }, "WalletTransfer": func() idHook { return &WalletTransfer{} },
	}
	for name, newModel := range models {
		t.Run(name, func(t *testing.T) {
			m := newModel()
			if err := m.BeforeCreate(nil); err != nil {
				t.Fatal(err)
			}
			id := idOf(m)
			if id == uuid.Nil || id.Version() != 7 {
				t.Errorf("expected a v7 id, got %s", id)
			}

			// An id set by the caller is kept.
			fixed := uuid.New()
			m = newModel()
			reflect.ValueOf(m).Elem().FieldByName("ID").Set(reflect.ValueOf(fixed))
			_ = m.BeforeCreate(nil)
			if idOf(m) != fixed {
				t.Errorf("existing id overwritten: %s", idOf(m))
			}
		})
	}
}

type idHook interface {
	BeforeCreate(*gorm.DB) error
}

func idOf(m idHook) uuid.UUID {
	return reflect.ValueOf(m).Elem().FieldByName("ID").Interface().(uuid.UUID)
}

func TestTableNames(t *testing.T) {
	if (RefreshToken{}).TableName() != "refresh_tokens" || (PushSubscription{}).TableName() != "push_subscriptions" {
		t.Error("unexpected table name")
	}
}
