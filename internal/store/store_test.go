package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kdubb1337/fin-cli/internal/config"
	"github.com/kdubb1337/fin-cli/internal/provider"
)

func openTmp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cache.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrateAdvancesUserVersion(t *testing.T) {
	s := openTmp(t)
	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected user_version=1, got %d", v)
	}
	got, err := s.IntegrityCheck()
	if err != nil || got != "ok" {
		t.Fatalf("integrity_check = %q (err=%v)", got, err)
	}
}

func TestUpsertTransactionIsIdempotent(t *testing.T) {
	s := openTmp(t)
	if err := s.UpsertItem("item_1", config.Item{Provider: "plaid", Env: "sandbox", AddedAt: time.Now()}); err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	tx := provider.Transaction{
		ID:        "tx_1",
		AccountID: "acct_1",
		Date:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		Amount:    12.34,
		Currency:  "USD",
		Name:      "Starbucks",
	}
	for i := 0; i < 3; i++ {
		if err := s.UpsertTransactions("item_1", []provider.Transaction{tx}); err != nil {
			t.Fatalf("upsert tx (run %d): %v", i, err)
		}
	}
	n, err := s.CountTransactions("item_1")
	if err != nil || n != 1 {
		t.Fatalf("count = %d (err=%v), want 1", n, err)
	}
	// Modify the row — same id, new name + amount.
	tx.Amount = 99
	tx.Name = "Starbucks Reserve"
	if err := s.UpsertTransactions("item_1", []provider.Transaction{tx}); err != nil {
		t.Fatalf("update upsert: %v", err)
	}
	rows, err := s.ListTransactions(TxQuery{ItemID: "item_1", Limit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("list = %d rows (err=%v)", len(rows), err)
	}
	if rows[0].Amount != 99 || rows[0].Name != "Starbucks Reserve" {
		t.Fatalf("row not updated: %+v", rows[0])
	}
}

func TestFTSMatchesName(t *testing.T) {
	s := openTmp(t)
	if err := s.UpsertItem("item_1", config.Item{Provider: "plaid", Env: "sandbox", AddedAt: time.Now()}); err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	txs := []provider.Transaction{
		{ID: "t1", AccountID: "a", Date: mustDate(t, "2026-01-01"), Amount: 5, Name: "Starbucks downtown"},
		{ID: "t2", AccountID: "a", Date: mustDate(t, "2026-01-02"), Amount: 6, Name: "Whole Foods Market"},
		{ID: "t3", AccountID: "a", Date: mustDate(t, "2026-01-03"), Amount: 7, Name: "Uber Eats", MerchantName: "Uber"},
	}
	if err := s.UpsertTransactions("item_1", txs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.ListTransactions(TxQuery{FTSMatch: "starbucks", Limit: 10})
	if err != nil {
		t.Fatalf("fts query: %v", err)
	}
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("fts match = %+v", got)
	}
	// Modifying a row should keep FTS in sync (triggers).
	txs[0].Name = "Peets Coffee"
	if err := s.UpsertTransactions("item_1", txs[:1]); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err = s.ListTransactions(TxQuery{FTSMatch: "starbucks", Limit: 10})
	if err != nil {
		t.Fatalf("fts re-query: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 rows after rename, got %d", len(got))
	}
}

func TestCursorRoundTrip(t *testing.T) {
	s := openTmp(t)
	if err := s.UpsertItem("item_1", config.Item{Provider: "plaid", Env: "sandbox", AddedAt: time.Now()}); err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	cur, err := s.GetCursor("item_1", "transactions")
	if err != nil || cur != "" {
		t.Fatalf("initial cursor = %q (err=%v)", cur, err)
	}
	if err := s.SetCursor("item_1", "transactions", "abc123"); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	cur, _ = s.GetCursor("item_1", "transactions")
	if cur != "abc123" {
		t.Fatalf("cursor = %q, want abc123", cur)
	}
	if err := s.ResetCursor("item_1", "transactions"); err != nil {
		t.Fatalf("reset cursor: %v", err)
	}
	cur, _ = s.GetCursor("item_1", "transactions")
	if cur != "" {
		t.Fatalf("post-reset cursor = %q, want empty", cur)
	}
}

func TestReadOnlyRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	rw, err := Open(path)
	if err != nil {
		t.Fatalf("open rw: %v", err)
	}
	_ = rw.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("open ro: %v", err)
	}
	defer ro.Close()
	if _, err := ro.DB.Exec(`INSERT INTO items (id, provider, env, added_at) VALUES ('x','p','sandbox','2026-01-01T00:00:00Z')`); err == nil {
		t.Fatalf("expected write to fail in read-only mode")
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}
