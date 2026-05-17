package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	"github.com/kdubb1337/fin-cli/internal/provider"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
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
)

var txCmd = &cobra.Command{
	Use:     "tx",
	Aliases: []string{"transactions"},
	Short:   "Query transactions",
}

var txListCmd = &cobra.Command{
	Use:   "list",
	Short: "List transactions on the resolved item",
	Example: `  fin tx list --since 2025-01-01 --limit 50
  fin transactions list --account acc_1234`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		// flagProfile is the global --profile from rootCmd (see root.go).
		itemID, err := c.ResolveItem(txItem, flagProfile, os.Getenv("FIN_PROFILE"))
		if err != nil {
			return finerr.New(finerr.CodeUsage, "%v", err)
		}
		tok, err := config.GetSecret("plaid:item:" + itemID)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "token: %v", err)
		}
		client, err := plaidprov.New(c)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "client: %v", err)
		}

		// flagAccount is the global --account from rootCmd.
		opts := provider.TxOptions{AccountID: flagAccount, Limit: txLimit, Cursor: txCursor}
		if txSince != "" {
			t, perr := time.Parse("2006-01-02", txSince)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--since must be YYYY-MM-DD")
			}
			opts.Since = t
		}
		if txUntil != "" {
			t, perr := time.Parse("2006-01-02", txUntil)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--until must be YYYY-MM-DD")
			}
			opts.Until = t
		}
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

func init() {
	txListCmd.Flags().StringVar(&txItem, "item", "", "item-id (overrides --profile)")
	txListCmd.Flags().StringVar(&txSince, "since", "", "start date YYYY-MM-DD (default: 1 month ago)")
	txListCmd.Flags().StringVar(&txUntil, "until", "", "end date YYYY-MM-DD (default: today)")
	txListCmd.Flags().IntVar(&txLimit, "limit", 25, "max transactions per page")
	txListCmd.Flags().StringVar(&txCursor, "cursor", "", "pagination cursor")
	txCmd.AddCommand(txListCmd)
	rootCmd.AddCommand(txCmd)
}
