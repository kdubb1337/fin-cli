package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersion = 1

type Config struct {
	SchemaVersion int                `json:"schema_version"`
	Plaid         PlaidConfig        `json:"plaid"`
	Items         map[string]Item    `json:"items"`
	Profiles      map[string]Profile `json:"profiles"`
	ActiveProfile string             `json:"active_profile"`
}

type PlaidConfig struct {
	ClientID string `json:"client_id"`
	Env      string `json:"env"`
}

type Item struct {
	Provider        string    `json:"provider"`
	Env             string    `json:"env"`
	InstitutionID   string    `json:"institution_id"`
	InstitutionName string    `json:"institution_name"`
	AddedAt         time.Time `json:"added_at"`
}

type Profile struct {
	ItemID string `json:"item_id"`
}

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fin"), nil
}

func path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{SchemaVersion: SchemaVersion, Items: map[string]Item{}, Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}
	if c.Items == nil {
		c.Items = map[string]Item{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}
	return &c, nil
}

func Save(c *Config) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	p := filepath.Join(d, "config.json")
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Resolve is a backward-compatible shim for the scaffold's PersistentPreRunE.
// The real precedence chain is implemented by (*Config).ResolveItem; this
// shim is a no-op so root-level command binding stays decoupled from item
// resolution, which subcommands perform on demand.
func Resolve(profile, account string) error {
	_ = profile
	_ = account
	return nil
}
