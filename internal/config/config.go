package config

import "os"

// Resolve applies the precedence:
//   explicit --profile flag > --account flag > $FIN_ACCOUNT env > stored default
//
// Stub for now — wire your real config-file loader here.
func Resolve(profile, account string) error {
	if account == "" {
		account = os.Getenv("FIN_ACCOUNT")
	}
	// TODO: read ~/.fin/config.yaml, resolve named profile, etc.
	_ = profile
	_ = account
	return nil
}
