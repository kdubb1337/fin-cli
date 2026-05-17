package store

import (
	"database/sql"
	"os"
	"time"

	"github.com/kdubb1337/fin-cli/internal/config"
)

// UpsertItem mirrors a config.Item into the items table. We don't carry
// last_synced_at here — that's owned by MarkItemSynced after a successful sync.
func (s *Store) UpsertItem(id string, it config.Item) error {
	_, err := s.DB.Exec(`
		INSERT INTO items (id, provider, env, institution_id, institution_name, added_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider         = excluded.provider,
			env              = excluded.env,
			institution_id   = excluded.institution_id,
			institution_name = excluded.institution_name,
			added_at         = excluded.added_at
	`, id, it.Provider, it.Env, it.InstitutionID, it.InstitutionName, it.AddedAt.UTC().Format(time.RFC3339))
	return err
}

// MarkItemSynced stamps last_synced_at to the given time (UTC).
func (s *Store) MarkItemSynced(itemID string, t time.Time) error {
	_, err := s.DB.Exec(`UPDATE items SET last_synced_at = ? WHERE id = ?`, t.UTC().Format(time.RFC3339), itemID)
	return err
}

// ItemSyncStatus is one row in `fin sync status`.
type ItemSyncStatus struct {
	ItemID           string `json:"item_id"`
	Provider         string `json:"provider"`
	Env              string `json:"env"`
	InstitutionName  string `json:"institution_name,omitempty"`
	LastSyncedAt     string `json:"last_synced_at,omitempty"`
	TransactionCount int64  `json:"transaction_count"`
	AccountCount     int64  `json:"account_count"`
	HasCursor        bool   `json:"has_cursor"`
}

func (s *Store) ListSyncStatus() ([]ItemSyncStatus, error) {
	rows, err := s.DB.Query(`
		SELECT
			i.id, i.provider, i.env, COALESCE(i.institution_name, ''), COALESCE(i.last_synced_at, ''),
			(SELECT COUNT(*) FROM transactions t WHERE t.item_id = i.id),
			(SELECT COUNT(*) FROM accounts    a WHERE a.item_id = i.id),
			EXISTS(SELECT 1 FROM sync_cursors c WHERE c.item_id = i.id AND c.resource = 'transactions' AND c.cursor IS NOT NULL AND c.cursor != '')
		FROM items i
		ORDER BY i.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ItemSyncStatus
	for rows.Next() {
		var r ItemSyncStatus
		if err := rows.Scan(&r.ItemID, &r.Provider, &r.Env, &r.InstitutionName, &r.LastSyncedAt, &r.TransactionCount, &r.AccountCount, &r.HasCursor); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetCursor returns the stored cursor for (itemID, resource), or "" when no
// row is present. Callers treat "no cursor" and "empty cursor" as equivalent
// (Plaid /transactions/sync accepts either to mean "from the beginning").
func (s *Store) GetCursor(itemID, resource string) (string, error) {
	var cur sql.NullString
	err := s.DB.QueryRow(`SELECT cursor FROM sync_cursors WHERE item_id = ? AND resource = ?`, itemID, resource).Scan(&cur)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cur.String, nil
}

// SetCursor upserts a cursor row.
func (s *Store) SetCursor(itemID, resource, cursor string) error {
	_, err := s.DB.Exec(`
		INSERT INTO sync_cursors (item_id, resource, cursor, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(item_id, resource) DO UPDATE SET
			cursor     = excluded.cursor,
			updated_at = excluded.updated_at
	`, itemID, resource, cursor, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ResetCursor wipes the stored cursor for an item+resource pair. Used by
// `fin sync --full` so the next sync re-mirrors from the beginning.
func (s *Store) ResetCursor(itemID, resource string) error {
	_, err := s.DB.Exec(`DELETE FROM sync_cursors WHERE item_id = ? AND resource = ?`, itemID, resource)
	return err
}

// DBStats is used by `fin doctor`.
type DBStats struct {
	Path             string `json:"path"`
	SchemaVersion    int    `json:"schema_version"`
	SizeBytes        int64  `json:"size_bytes"`
	ItemCount        int64  `json:"item_count"`
	AccountCount     int64  `json:"account_count"`
	TransactionCount int64  `json:"transaction_count"`
}

func (s *Store) Stats() (DBStats, error) {
	out := DBStats{Path: s.Path}
	v, err := s.SchemaVersion()
	if err != nil {
		return out, err
	}
	out.SchemaVersion = v
	if info, err := os.Stat(s.Path); err == nil {
		out.SizeBytes = info.Size()
	}
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&out.ItemCount)
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&out.AccountCount)
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&out.TransactionCount)
	return out, nil
}
