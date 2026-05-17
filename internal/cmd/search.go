package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	"github.com/kdubb1337/fin-cli/internal/store"
)

var (
	searchItem    string
	searchAccount string
	searchSince   string
	searchUntil   string
	searchLimit   int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search over cached transactions (FTS5)",
	Long: `Searches the transaction name, merchant_name, and category fields using
SQLite FTS5. The <query> is passed through to FTS5 as-is, so all match operators
(prefix*, AND/OR/NOT, "phrase", ^) work.

Requires a synced cache; run "fin sync" first.`,
	Example: `  fin search "starbucks"
  fin search "uber OR lyft" --since 2025-01-01
  fin search "amazon*" --account acc_abc --limit 50`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.TrimSpace(strings.Join(args, " "))
		if q == "" {
			return finerr.New(finerr.CodeUsage, "search query is required")
		}

		s, err := store.Open("")
		if err != nil {
			if os.IsNotExist(err) {
				return finerr.New(finerr.CodeNotFound, "cache db not found; run `fin sync` first")
			}
			return finerr.Wrap(err, finerr.CodeGeneric, "store: %v", err)
		}
		defer s.Close()

		txq := store.TxQuery{
			ItemID:    searchItem,
			AccountID: searchAccount,
			Limit:     searchLimit,
			FTSMatch:  q,
		}
		if searchSince != "" {
			t, perr := time.Parse("2006-01-02", searchSince)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--since must be YYYY-MM-DD")
			}
			txq.Since = t
		}
		if searchUntil != "" {
			t, perr := time.Parse("2006-01-02", searchUntil)
			if perr != nil {
				return finerr.New(finerr.CodeUsage, "--until must be YYYY-MM-DD")
			}
			txq.Until = t
		}
		results, err := s.ListTransactions(txq)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeValidation, "fts query: %v", err)
		}
		// ListTransactions over-fetches by 1 to detect truncation.
		truncated := false
		if len(results) > searchLimit && searchLimit > 0 {
			results = results[:searchLimit]
			truncated = true
		}
		hint := ""
		if truncated {
			hint = "more results available; refine with --since/--account or raise --limit"
		}
		return output.EmitPage(results, "", hint)
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchItem, "item", "", "restrict to a single item")
	searchCmd.Flags().StringVar(&searchAccount, "account", "", "restrict to a single account")
	searchCmd.Flags().StringVar(&searchSince, "since", "", "start date YYYY-MM-DD")
	searchCmd.Flags().StringVar(&searchUntil, "until", "", "end date YYYY-MM-DD")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 25, "max results")
	rootCmd.AddCommand(searchCmd)
}
