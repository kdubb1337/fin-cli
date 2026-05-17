package store

import (
	"time"

	"github.com/kdubb1337/fin-cli/internal/provider"
)

// UpsertAccount mirrors one provider.Account row into the accounts table. The
// caller is responsible for setting Account.ItemID — the live API responses
// don't carry it.
func (s *Store) UpsertAccount(a provider.Account) error {
	var avail any
	if a.AvailableBalance != nil {
		avail = *a.AvailableBalance
	}
	_, err := s.DB.Exec(`
		INSERT INTO accounts (id, item_id, name, official_name, mask, type, subtype, currency, balance, available_balance, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			item_id           = excluded.item_id,
			name              = excluded.name,
			official_name     = excluded.official_name,
			mask              = excluded.mask,
			type              = excluded.type,
			subtype           = excluded.subtype,
			currency          = excluded.currency,
			balance           = excluded.balance,
			available_balance = excluded.available_balance,
			updated_at        = excluded.updated_at
	`, a.ID, a.ItemID, a.Name, a.OfficialName, a.Mask, a.Type, a.Subtype, a.Currency, a.Balance, avail, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ListAccounts returns accounts for itemID, or all accounts when itemID is "".
func (s *Store) ListAccounts(itemID string) ([]provider.Account, error) {
	q := `SELECT id, item_id, name, COALESCE(official_name,''), COALESCE(mask,''),
	             COALESCE(type,''), COALESCE(subtype,''), COALESCE(currency,''),
	             COALESCE(balance,0), available_balance
	      FROM accounts`
	args := []any{}
	if itemID != "" {
		q += ` WHERE item_id = ?`
		args = append(args, itemID)
	}
	q += ` ORDER BY name`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []provider.Account
	for rows.Next() {
		var a provider.Account
		var avail any
		if err := rows.Scan(&a.ID, &a.ItemID, &a.Name, &a.OfficialName, &a.Mask, &a.Type, &a.Subtype, &a.Currency, &a.Balance, &avail); err != nil {
			return nil, err
		}
		if avail != nil {
			if f, ok := avail.(float64); ok {
				v := f
				a.AvailableBalance = &v
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
