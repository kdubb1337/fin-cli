package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("FIN_KEYRING_BACKEND", "file")

	c := &Config{
		SchemaVersion: 1,
		Plaid:         PlaidConfig{ClientID: "test_id", Env: "sandbox"},
		Items: map[string]Item{
			"access-sandbox-abc": {
				Provider: "plaid", Env: "sandbox",
				InstitutionID: "ins_56", InstitutionName: "RBC",
			},
		},
		Profiles:      map[string]Profile{"default": {ItemID: "access-sandbox-abc"}},
		ActiveProfile: "default",
	}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, ".fin", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("want mode 0600, got %v", info.Mode().Perm())
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Plaid.ClientID != "test_id" {
		t.Errorf("got %q", got.Plaid.ClientID)
	}
	if len(got.Items) != 1 {
		t.Errorf("got %d items", len(got.Items))
	}
}

func TestSecretRoundTripFileBackend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("FIN_KEYRING_BACKEND", "file")

	if err := StoreSecret("plaid:client_secret", "shh"); err != nil {
		t.Fatal(err)
	}
	got, err := GetSecret("plaid:client_secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "shh" {
		t.Errorf("got %q", got)
	}
}
