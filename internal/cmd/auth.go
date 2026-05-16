package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/oauthflow"
	"github.com/kdubb1337/fin-cli/internal/output"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
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

// auth add walks the user through Plaid Link to attach a new institution. It
// runs a local callback listener, opens a browser to link.plaid.com, exchanges
// the public_token for a permanent access_token, persists item metadata to
// ~/.fin/config.json, and stores the access_token in the keychain.

var (
	addEnv       string
	addPublicTok string
)

var authAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Link a new bank or brokerage via Plaid",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		if addEnv == "" {
			addEnv = c.Plaid.Env
		}
		if addEnv == "" {
			return finerr.New(finerr.CodeUsage, "no env configured; run `fin auth setup` first")
		}
		c.Plaid.Env = addEnv

		client, err := plaidprov.New(c)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeAuth, "plaid client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		var publicToken, instID, instName string

		if addPublicTok != "" {
			publicToken = addPublicTok
		} else {
			if flagNoInput {
				return finerr.New(finerr.CodeUsage, "--no-input requires --public-token (capture one via Plaid Quickstart)")
			}
			port := 53682
			if v := os.Getenv("FIN_OAUTH_PORT"); v != "" {
				if n, perr := strconv.Atoi(v); perr == nil {
					port = n
				}
			}
			l, err := oauthflow.Start(ctx, port)
			if err != nil {
				return finerr.Wrap(err, finerr.CodeGeneric, "callback listener: %v", err)
			}

			session, err := client.StartLink(ctx, l.URL()+"/callback")
			if err != nil {
				return err
			}

			linkURL := "https://link.plaid.com/?token=" + session.Token
			fmt.Fprintf(cmd.ErrOrStderr(), "Opening Plaid Link in browser…\nIf nothing happens, visit: %s\n", linkURL)
			_ = browser.OpenURL(linkURL)

			res, werr := l.Wait()
			if werr != nil {
				return finerr.Wrap(werr, finerr.CodeTimeout, "%v", werr)
			}
			publicToken = res.PublicToken
			instID = res.InstitutionID
			instName = res.InstitutionName
		}

		ex, err := client.ExchangePublicToken(ctx, publicToken)
		if err != nil {
			return err
		}
		if instID == "" {
			instID = ex.InstitutionID
		}
		if instName == "" {
			instName = ex.InstitutionName
		}

		if err := config.StoreSecret("plaid:item:"+ex.ItemID, ex.AccessToken); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "keychain: %v", err)
		}
		c.Items[ex.ItemID] = config.Item{
			Provider:        "plaid",
			Env:             addEnv,
			InstitutionID:   instID,
			InstitutionName: instName,
			AddedAt:         time.Now().UTC(),
		}
		// Create a default profile if none exists yet
		if len(c.Profiles) == 0 {
			c.Profiles = map[string]config.Profile{"default": {ItemID: ex.ItemID}}
			c.ActiveProfile = "default"
		}
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}

		return output.Emit(map[string]string{
			"status":           "linked",
			"item_id":          ex.ItemID,
			"institution_id":   instID,
			"institution_name": instName,
			"env":              addEnv,
		})
	},
}

func init() {
	authSetupCmd.Flags().StringVar(&setupClientID, "client-id", "", "Plaid client_id")
	authSetupCmd.Flags().StringVar(&setupSecret, "secret", "", "Plaid secret")
	authSetupCmd.Flags().StringVar(&setupEnv, "env", "sandbox", "sandbox or production")
	authCmd.AddCommand(authSetupCmd)

	authAddCmd.Flags().StringVar(&addEnv, "env", "", "sandbox or production (defaults to value from `fin auth setup`)")
	authAddCmd.Flags().StringVar(&addPublicTok, "public-token", "", "skip browser flow; exchange this public_token directly")
	authCmd.AddCommand(authAddCmd)
}
