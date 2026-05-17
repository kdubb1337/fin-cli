package cmd

import (
	"context"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	"github.com/kdubb1337/fin-cli/internal/provider"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
	"github.com/kdubb1337/fin-cli/internal/store"
)

var (
	syncItem    string
	syncFull    bool
	syncMaxIter int
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Mirror upstream accounts and transactions into the local cache",
	Long: `Walks every linked item (or just --item) and pulls deltas from the provider
into ~/.fin/cache.db. Uses Plaid /transactions/sync (cursor-based) so each run
fetches only what changed since the last call.

Use --full to wipe the cursor first and re-mirror everything.
Use --dry-run to preview without writing.`,
	Example: `  fin sync
  fin sync --item item_abc --full
  fin sync --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		if len(c.Items) == 0 {
			return finerr.New(finerr.CodeUsage, "no items linked; run `fin auth add`")
		}

		s, err := store.Open("")
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "store: %v", err)
		}
		defer s.Close()

		ids := make([]string, 0, len(c.Items))
		if syncItem != "" {
			if _, ok := c.Items[syncItem]; !ok {
				return finerr.New(finerr.CodeNotFound, "item %q not linked", syncItem)
			}
			ids = append(ids, syncItem)
		} else {
			for id := range c.Items {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)

		type result struct {
			ItemID      string `json:"item_id"`
			Institution string `json:"institution_name,omitempty"`
			Accounts    int    `json:"accounts"`
			Added       int    `json:"added"`
			Modified    int    `json:"modified"`
			Removed     int    `json:"removed"`
			Pages       int    `json:"pages"`
			NextCursor  string `json:"next_cursor,omitempty"`
			Status      string `json:"status"`
			Error       string `json:"error,omitempty"`
		}
		results := make([]result, 0, len(ids))

		var firstErr error
		for _, id := range ids {
			it := c.Items[id]
			r := result{ItemID: id, Institution: it.InstitutionName, Status: "ok"}

			if flagDryRun {
				cur, _ := s.GetCursor(id, "transactions")
				if syncFull {
					cur = ""
				}
				r.Status = "would-sync"
				r.NextCursor = cur
				results = append(results, r)
				continue
			}

			if syncFull {
				if err := s.ResetCursor(id, "transactions"); err != nil {
					r.Status = "error"
					r.Error = err.Error()
					results = append(results, r)
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
			}
			if err := s.UpsertItem(id, it); err != nil {
				r.Status = "error"
				r.Error = err.Error()
				results = append(results, r)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}

			client, err := plaidprov.NewForEnv(c, it.Env)
			if err != nil {
				r.Status = "error"
				r.Error = err.Error()
				results = append(results, r)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			added, modified, removed, pages, cursor, err := syncOne(ctx, client, s, id, &it, syncMaxIter)
			cancel()

			r.Accounts = countAccounts(s, id)
			r.Added, r.Modified, r.Removed, r.Pages, r.NextCursor = added, modified, removed, pages, cursor
			if err != nil {
				r.Status = "error"
				r.Error = err.Error()
				if firstErr == nil {
					firstErr = err
				}
			}
			results = append(results, r)
		}

		payload := map[string]any{
			"items":   results,
			"dry_run": flagDryRun,
		}
		if flagDryRun {
			_ = output.EmitDryRun(payload)
		} else {
			_ = output.Emit(payload)
		}
		return firstErr
	},
}

// syncOne refreshes accounts + transactions for a single item. Returns delta
// counts, page count, and the cursor we ended on. The cursor is persisted at
// the end of each page so a crash mid-loop resumes cleanly.
func syncOne(ctx context.Context, client provider.Provider, s *store.Store, itemID string, it *config.Item, maxIter int) (int, int, int, int, string, error) {
	tok, err := config.GetSecret("plaid:item:" + itemID)
	if err != nil {
		return 0, 0, 0, 0, "", finerr.Wrap(err, finerr.CodeAuth, "token: %v", err)
	}

	// Account snapshot — `/accounts/get` is cheap, do this on every sync so
	// balances stay fresh in the cache.
	accts, err := client.ListAccounts(ctx, tok)
	if err != nil {
		return 0, 0, 0, 0, "", err
	}
	for _, a := range accts {
		a.ItemID = itemID
		a.InstitutionName = it.InstitutionName
		if err := s.UpsertAccount(a); err != nil {
			return 0, 0, 0, 0, "", err
		}
	}

	cursor, err := s.GetCursor(itemID, "transactions")
	if err != nil {
		return 0, 0, 0, 0, cursor, err
	}

	addedN, modN, remN, pages := 0, 0, 0, 0
	for {
		if maxIter > 0 && pages >= maxIter {
			break
		}
		page, err := client.SyncTransactions(ctx, tok, cursor)
		if err != nil {
			return addedN, modN, remN, pages, cursor, err
		}
		pages++
		// Upsert added + modified in one batch; ON CONFLICT DO UPDATE handles both.
		if err := s.UpsertTransactions(itemID, page.Added); err != nil {
			return addedN, modN, remN, pages, cursor, err
		}
		if err := s.UpsertTransactions(itemID, page.Modified); err != nil {
			return addedN, modN, remN, pages, cursor, err
		}
		if err := s.DeleteTransactions(page.Removed); err != nil {
			return addedN, modN, remN, pages, cursor, err
		}
		addedN += len(page.Added)
		modN += len(page.Modified)
		remN += len(page.Removed)

		// Persist cursor *after* the page commit so a crash resumes here.
		if err := s.SetCursor(itemID, "transactions", page.NextCursor); err != nil {
			return addedN, modN, remN, pages, cursor, err
		}
		cursor = page.NextCursor
		if !page.HasMore {
			break
		}
	}

	if err := s.MarkItemSynced(itemID, time.Now()); err != nil {
		return addedN, modN, remN, pages, cursor, err
	}
	return addedN, modN, remN, pages, cursor, nil
}

func countAccounts(s *store.Store, itemID string) int {
	accts, err := s.ListAccounts(itemID)
	if err != nil {
		return 0
	}
	return len(accts)
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show last sync time, row counts, and cursor presence per item",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Open("")
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "store: %v", err)
		}
		defer s.Close()
		rows, err := s.ListSyncStatus()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "query: %v", err)
		}
		return output.Emit(rows)
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncItem, "item", "", "sync only this item-id (default: all linked items)")
	syncCmd.Flags().BoolVar(&syncFull, "full", false, "reset the cursor and re-mirror everything")
	syncCmd.Flags().IntVar(&syncMaxIter, "max-pages", 0, "cap pages per item (0 = until has_more is false)")
	syncCmd.AddCommand(syncStatusCmd)
	rootCmd.AddCommand(syncCmd)
}
