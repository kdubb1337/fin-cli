package oauthflow

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestCallbackHappyPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l, err := Start(ctx, 0, "") // 0 → ephemeral port; no link token needed for direct /callback POST
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		//nolint:errcheck,gosec
		http.Get(l.URL() + "/callback?public_token=pub-sandbox-xyz&institution_id=ins_56&institution_name=RBC")
	}()
	res, err := l.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if res.PublicToken != "pub-sandbox-xyz" {
		t.Fatalf("got %q", res.PublicToken)
	}
	if res.InstitutionID != "ins_56" {
		t.Fatalf("got %q", res.InstitutionID)
	}
}

func TestCallbackTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	l, err := Start(ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Wait(); err == nil {
		t.Fatal("expected timeout error")
	}
}
