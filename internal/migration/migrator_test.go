package migration

import "testing"

// The refusal happens before the DB is touched, so no database is needed.
func TestAutoMigrateRefusedInProduction(t *testing.T) {
	for _, env := range []string{"production", "prod"} {
		t.Setenv("APP_ENV", env)
		if err := NewMigrator(nil).AutoMigrate(); err == nil {
			t.Fatalf("APP_ENV=%s: expected AutoMigrate to be refused", env)
		}
	}
}
