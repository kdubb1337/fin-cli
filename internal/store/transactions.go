package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/kdubb1337/fin-cli/internal/provider"
)

// UpsertTransactions writes a batch in one transaction. Plaid /transactions/sync
// returns the same row in "added" and "modified" across windows; upsert is the
// only correct shape.
func (s *Store) UpsertTransactions(itemID string, txs []provider.Transaction) error {
	if len(txs) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO transactions (id, item_id, account_id, date, amount, currency, name, merchant_name, pending, category_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			item_id       = excluded.item_id,
			account_id    = excluded.account_id,
			date          = excluded.date,
			amount        = excluded.amount,
			currency      = excluded.currency,
			name          = excluded.name,
			merchant_name = excluded.merchant_name,
			pending       = excluded.pending,
			category_json = excluded.category_json,
			updated_at    = excluded.updated_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range txs {
		var catJSON string
		if len(t.Category) > 0 {
			b, _ := json.Marshal(t.Category)
			catJSON = string(b)
		}
		pending := 0
		if t.Pending {
			pending = 1
		}
		if _, err := stmt.Exec(t.ID, itemID, t.AccountID, t.Date.Format("2006-01-02"),
			t.Amount, t.Currency, t.Name, t.MerchantName, pending, catJSON, now); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// DeleteTransactions removes by id. Plaid /transactions/sync returns ids in
// the "removed" slice; we honor it so the cache matches upstream truth.
func (s *Store) DeleteTransactions(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`DELETE FROM transactions WHERE id = ?`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// TxQuery is the filter set shared by cached list + search.
type TxQuery struct {
	ItemID    string
	AccountID string
	Since     time.Time
	Until     time.Time
	Limit     int
	Offset    int
	FTSMatch  string // when non-empty, joins through transactions_fts
}

// ListTransactions runs a filtered query against the cached transactions table.
func (s *Store) ListTransactions(q TxQuery) ([]provider.Transaction, error) {
	var where []string
	var args []any

	base := `SELECT t.id, t.item_id, t.account_id, t.date, t.amount, COALESCE(t.currency,''),
	                COALESCE(t.name,''), COALESCE(t.merchant_name,''), t.pending, COALESCE(t.category_json,'')
	         FROM transactions t`
	if q.FTSMatch != "" {
		base += ` JOIN transactions_fts f ON f.rowid = t.rowid`
		where = append(where, `transactions_fts MATCH ?`)
		args = append(args, q.FTSMatch)
	}
	if q.ItemID != "" {
		where = append(where, `t.item_id = ?`)
		args = append(args, q.ItemID)
	}
	if q.AccountID != "" {
		where = append(where, `t.account_id = ?`)
		args = append(args, q.AccountID)
	}
	if !q.Since.IsZero() {
		where = append(where, `t.date >= ?`)
		args = append(args, q.Since.Format("2006-01-02"))
	}
	if !q.Until.IsZero() {
		where = append(where, `t.date <= ?`)
		args = append(args, q.Until.Format("2006-01-02"))
	}
	if len(where) > 0 {
		base += " WHERE " + strings.Join(where, " AND ")
	}
	base += ` ORDER BY t.date DESC, t.id DESC`
	limit := q.Limit
	if limit <= 0 {
		limit = 25
	}
	base += ` LIMIT ?`
	args = append(args, limit+1) // +1 to detect "has more"
	if q.Offset > 0 {
		base += ` OFFSET ?`
		args = append(args, q.Offset)
	}

	rows, err := s.DB.Query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []provider.Transaction
	for rows.Next() {
		var t provider.Transaction
		var dateStr, catJSON string
		var pending int
		var itemID string
		if err := rows.Scan(&t.ID, &itemID, &t.AccountID, &dateStr, &t.Amount, &t.Currency,
			&t.Name, &t.MerchantName, &pending, &catJSON); err != nil {
			return nil, err
		}
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			t.Date = d
		}
		t.Pending = pending != 0
		if catJSON != "" {
			_ = json.Unmarshal([]byte(catJSON), &t.Category)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountTransactions returns the row count for an item (used by sync status /
// doctor).
func (s *Store) CountTransactions(itemID string) (int64, error) {
	var n int64
	q := `SELECT COUNT(*) FROM transactions`
	if itemID == "" {
		err := s.DB.QueryRow(q).Scan(&n)
		return n, err
	}
	err := s.DB.QueryRow(q+` WHERE item_id = ?`, itemID).Scan(&n)
	return n, err
}

// LastSyncedAt returns the latest items.last_synced_at across all items as a
// time.Time (or zero when no items exist / none synced).
func (s *Store) LastSyncedAt() (time.Time, error) {
	var v sql.NullString
	if err := s.DB.QueryRow(`SELECT MAX(last_synced_at) FROM items`).Scan(&v); err != nil {
		return time.Time{}, err
	}
	if !v.Valid || v.String == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v.String)
}
