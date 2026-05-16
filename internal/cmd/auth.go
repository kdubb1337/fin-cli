package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
)

// auth setup configures the user's Plaid client_id and secret. The client_id
// and env are persisted in ~/.fin/config.json; the secret goes to the OS
// keychain (or the file backend when FIN_KEYRING_BACKEND=file).

var (
	setupClientID string
	setupSecret   string
	setupEnv      string
)

var authSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure Plaid client_id and secret",
	Example: `  fin auth setup --client-id <id> --secret <secret> --env sandbox
  fin auth setup --client-id <id> --secret <secret> --env production --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if setupClientID == "" || setupSecret == "" {
			return finerr.New(finerr.CodeUsage,
				"--client-id and --secret are required (get them at https://dashboard.plaid.com/team/keys)")
		}
		if setupEnv != "sandbox" && setupEnv != "production" {
			return finerr.New(finerr.CodeUsage, "--env must be sandbox or production")
		}

		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config load: %v", err)
		}
		c.Plaid.ClientID = setupClientID
		c.Plaid.Env = setupEnv
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config save: %v", err)
		}
		if err := config.StoreSecret("plaid:client_secret", setupSecret); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "keychain store: %v", err)
		}

		prefix := setupClientID
		if len(setupClientID) > 6 {
			prefix = setupClientID[:6] + "…"
		}

		return output.Emit(map[string]any{
			"status":           "ok",
			"env":              setupEnv,
			"client_id_prefix": prefix,
		})
	},
}

func init() {
	authSetupCmd.Flags().StringVar(&setupClientID, "client-id", "", "Plaid client_id")
	authSetupCmd.Flags().StringVar(&setupSecret, "secret", "", "Plaid secret")
	authSetupCmd.Flags().StringVar(&setupEnv, "env", "sandbox", "sandbox or production")
	authCmd.AddCommand(authSetupCmd)
}
