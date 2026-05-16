package plaid

import (
	"testing"

	finerr "github.com/kdubb1337/fin-cli/internal/errors"
)

func TestMapErrorCode(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"ITEM_LOGIN_REQUIRED", finerr.CodeAuth},
		{"INVALID_ACCESS_TOKEN", finerr.CodeAuth},
		{"RATE_LIMIT_EXCEEDED", finerr.CodeRateLimited},
		{"PRODUCT_NOT_READY", finerr.CodeUpstream},
		{"INVALID_FIELD", finerr.CodeValidation},
		{"NO_ACCOUNTS", finerr.CodeNotFound},
		{"INTERNAL_SERVER_ERROR", finerr.CodeUpstream},
		{"WAT", finerr.CodeGeneric},
	}
	for _, c := range cases {
		if got := mapErrorCode(c.in); got != c.want {
			t.Errorf("%s: got %d want %d", c.in, got, c.want)
		}
	}
}
