package cmd

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	"github.com/kdubb1337/fin-cli/internal/provider"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
	"github.com/kdubb1337/fin-cli/internal/store"
)

// Local flags for `fin tx list`. --profile and --account are global persistent
// flags on rootCmd (flagProfile, flagAccount in root.go); we read them directly
// to avoid shadowing.
var (
	txItem   string
	txSince  string
	txUntil  string
	txLimit  int
	txCursor string
	txLive   bool
)

var txCmd = &cobra.Command{
	Use:     "tx",
	Aliases: []string{"transactions"},
	Short:   "Query transactions",
}

var txListCmd = &cobra.Command{
	Use:   "list",
	Short: "List transactions (cached by default; --live hits Plaid)",
	Long: `Lists transactions for the resolved item.

By default this reads from the local SQLite cache populated by ` + "`fin sync`" + `.
Pass --live to bypass the cache and call Plaid directly (counts against
your Plaid API quota; helpful when the cache is empty or stale).`,
	Example: `  fin tx list --since 2025-01-01 --limit 50
  fin transactions list --account acc_1234
  fin tx list --live`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		itemID, err := c.ResolveItem(txItem, flagProfile, os.Getenv("FIN_PROFILE"))
		if err != nil {
			return finerr.New(finerr.CodeUsage, "%v", err)
		}

		var since, until time.Time
		if txSince != "" {
			t, perr := time.Parse("2006-01-02", txSince)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--since must be YYYY-MM-DD")
			}
			since = t
		}
		if txUntil != "" {
			t, perr := time.Parse("2006-01-02", txUntil)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--until must be YYYY-MM-DD")
			}
			until = t
		}

		if !txLive {
			items, hint, err := cachedTxList(itemID, flagAccount, since, until, txLimit, txCursor)
			if err == nil {
				return output.EmitPage(items, hint, hintForCursor(hint))
			}
			if !os.IsNotExist(err) {
				return finerr.Wrap(err, finerr.CodeGeneric, "cache read: %v", err)
			}
			output.Progress("cache miss; falling back to live API (run `fin sync` to populate)")
		}

		tok, err := config.GetSecret("plaid:item:" + itemID)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "token: %v", err)
		}
		client, err := plaidprov.NewForEnv(c, c.Items[itemID].Env)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "client: %v", err)
		}
		opts := provider.TxOptions{AccountID: flagAccount, Limit: txLimit, Cursor: txCursor, Since: since, Until: until}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		page, err := client.ListTransactions(ctx, tok, opts)
		if err != nil {
			return err
		}
		var hint string
		if page.NextCursor != "" {
			hint = "next page: --cursor=" + page.NextCursor
		}
		return output.EmitPage(page.Transactions, page.NextCursor, hint)
	},
}

// cachedTxList reads from the local SQLite mirror. Returns os.ErrNotExist when
// the cache file is missing so the caller can fall back to a live call.
func cachedTxList(itemID, accountID string, since, until time.Time, limit int, cursor string) ([]provider.Transaction, string, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, "", err
	}
	s, err := store.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer s.Close()

	if limit <= 0 {
		limit = 25
	}
	offset := 0
	if cursor != "" {
		n, perr := strconv.Atoi(cursor)
		if perr != nil || n < 0 {
			return nil, "", finerr.New(finerr.CodeUsage, "cursor must be a non-negative integer")
		}
		offset = n
	}
	rows, err := s.ListTransactions(store.TxQuery{
		ItemID:    itemID,
		AccountID: accountID,
		Since:     since,
		Until:     until,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return rows, next, nil
}

func hintForCursor(cursor string) string {
	if cursor == "" {
		return ""
	}
	return "next page: --cursor=" + cursor
}

func init() {
	txListCmd.Flags().StringVar(&txItem, "item", "", "item-id (overrides --profile)")
	txListCmd.Flags().StringVar(&txSince, "since", "", "start date YYYY-MM-DD (default: 1 month ago)")
	txListCmd.Flags().StringVar(&txUntil, "until", "", "end date YYYY-MM-DD (default: today)")
	txListCmd.Flags().IntVar(&txLimit, "limit", 25, "max transactions per page")
	txListCmd.Flags().StringVar(&txCursor, "cursor", "", "pagination cursor")
	txListCmd.Flags().BoolVar(&txLive, "live", false, "skip the cache and hit Plaid directly")
	txCmd.AddCommand(txListCmd)
	rootCmd.AddCommand(txCmd)
}
