package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const service = "fin"

func StoreSecret(key, value string) error {
	if os.Getenv("FIN_KEYRING_BACKEND") == "file" {
		return fileStore(key, value)
	}
	return keyring.Set(service, key, value)
}

func GetSecret(key string) (string, error) {
	if os.Getenv("FIN_KEYRING_BACKEND") == "file" {
		return fileLoad(key)
	}
	v, err := keyring.Get(service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return v, err
}

func DeleteSecret(key string) error {
	if os.Getenv("FIN_KEYRING_BACKEND") == "file" {
		return fileDelete(key)
	}
	return keyring.Delete(service, key)
}

func fileBackendPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "keyring.json"), nil
}

func fileStore(key, value string) error {
	m, _ := fileAll()
	if m == nil {
		m = map[string]string{}
	}
	m[key] = value
	return fileWrite(m)
}

func fileLoad(key string) (string, error) {
	m, err := fileAll()
	if err != nil {
		return "", err
	}
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return v, nil
}

func fileDelete(key string) error {
	m, err := fileAll()
	if err != nil {
		return err
	}
	delete(m, key)
	return fileWrite(m)
}

func fileAll() (map[string]string, error) {
	p, err := fileBackendPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func fileWrite(m map[string]string) error {
	p, err := fileBackendPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
