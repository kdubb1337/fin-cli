package errors

import (
	"errors"
	"testing"
)

func TestExitCodeUnwrap(t *testing.T) {
	inner := errors.New("upstream boom")
	e := Wrap(inner, CodeUpstream, "Plaid 502")
	if e.Code() != CodeUpstream {
		t.Fatalf("got %d", e.Code())
	}
	if !errors.Is(e, inner) {
		t.Fatal("Is(inner) should be true")
	}
}
