package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
)

// --item is local to the accounts subtree. --profile is the global persistent
// flag on rootCmd (flagProfile in root.go); we read it here directly to avoid
// shadowing.
var acctFlagItem string

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Inspect linked bank/brokerage accounts",
}

var accountsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all accounts on the resolved item",
	Example: `  fin accounts list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		itemID, err := c.ResolveItem(acctFlagItem, flagProfile, os.Getenv("FIN_PROFILE"))
		if err != nil {
			return finerr.New(finerr.CodeUsage, "%v", err)
		}

		tok, err := config.GetSecret("plaid:item:" + itemID)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "token: %v", err)
		}

		client, err := plaidprov.NewForEnv(c, c.Items[itemID].Env)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "client: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		accts, err := client.ListAccounts(ctx, tok)
		if err != nil {
			return err
		}
		inst := c.Items[itemID].InstitutionName
		for i := range accts {
			accts[i].InstitutionName = inst
			accts[i].ItemID = itemID
		}
		return output.Emit(accts)
	},
}

var accountsGetCmd = &cobra.Command{
	Use:     "get <account-id>",
	Short:   "Show a single account by id",
	Example: `  fin accounts get acc_1234`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		itemID, err := c.ResolveItem(acctFlagItem, flagProfile, os.Getenv("FIN_PROFILE"))
		if err != nil {
			return finerr.New(finerr.CodeUsage, "%v", err)
		}
		tok, err := config.GetSecret("plaid:item:" + itemID)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "token: %v", err)
		}
		client, err := plaidprov.NewForEnv(c, c.Items[itemID].Env)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "client: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a, err := client.GetAccount(ctx, tok, args[0])
		if err != nil {
			return err
		}
		a.InstitutionName = c.Items[itemID].InstitutionName
		a.ItemID = itemID
		return output.Emit(a)
	},
}

func init() {
	for _, c := range []*cobra.Command{accountsListCmd, accountsGetCmd} {
		c.Flags().StringVar(&acctFlagItem, "item", "", "item-id to use (overrides --profile)")
	}
	accountsCmd.AddCommand(accountsListCmd, accountsGetCmd)
	rootCmd.AddCommand(accountsCmd)
}
