package plaid

import (
	"encoding/json"
	"errors"

	plaid "github.com/plaid/plaid-go/v25/plaid"

	finerr "github.com/kdubb1337/fin-cli/internal/errors"
)

func mapErrorCode(code string) int {
	switch code {
	case "ITEM_LOGIN_REQUIRED", "INVALID_ACCESS_TOKEN", "INVALID_API_KEYS",
		"UNAUTHORIZED", "ITEM_NOT_FOUND_ERROR":
		return finerr.CodeAuth
	case "RATE_LIMIT_EXCEEDED":
		return finerr.CodeRateLimited
	case "INVALID_FIELD", "INVALID_INPUT", "INVALID_BODY":
		return finerr.CodeValidation
	case "NO_ACCOUNTS", "PRODUCTS_NOT_SUPPORTED", "ITEM_NOT_SUPPORTED":
		return finerr.CodeNotFound
	case "INTERNAL_SERVER_ERROR", "PLANNED_MAINTENANCE", "PRODUCT_NOT_READY":
		return finerr.CodeUpstream
	default:
		return finerr.CodeGeneric
	}
}

// translateErr converts a Plaid SDK error into an ExitError with remediation hints.
// First call sites land in Tasks 7 and 8 (link.go, accounts.go, transactions.go).
//
//nolint:unused // wired up by Tasks 7/8
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	var pe plaid.GenericOpenAPIError
	if !errors.As(err, &pe) {
		return finerr.Wrap(err, finerr.CodeNetwork, "plaid: %v", err)
	}
	var body struct {
		ErrorCode      string `json:"error_code"`
		ErrorMessage   string `json:"error_message"`
		DisplayMessage string `json:"display_message"`
	}
	_ = json.Unmarshal(pe.Body(), &body)
	code := mapErrorCode(body.ErrorCode)
	msg := body.ErrorMessage
	if msg == "" {
		msg = err.Error()
	}
	if body.ErrorCode == "ITEM_LOGIN_REQUIRED" {
		msg += " — run `fin auth add` to re-link this institution"
	}
	return finerr.Wrap(err, code, "plaid: %s (%s)", msg, body.ErrorCode)
}
