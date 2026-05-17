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
	addUse       bool
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

		client, err := plaidprov.NewForEnv(c, addEnv)
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

			// Plaid's normal Link is JS-SDK-only — there's no top-level URL
			// to open. We mint a link_token, then serve a tiny local HTML
			// page that loads link-initialize.js and forwards the
			// public_token back to /callback on the same listener.
			// redirect_uri is left empty: it's only required for OAuth
			// institutions (production banks that bounce through the bank
			// site), not for sandbox or non-OAuth flows.
			session, err := client.StartLink(ctx, "")
			if err != nil {
				return err
			}

			l, err := oauthflow.Start(ctx, port, session.Token)
			if err != nil {
				return finerr.Wrap(err, finerr.CodeGeneric, "callback listener: %v", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Opening Plaid Link in browser…\nIf nothing happens, visit: %s\n", l.URL())
			_ = browser.OpenURL(l.URL())

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
		// Create a default profile if none exists yet, or repoint it when --use is set.
		profileAction := "unchanged"
		switch {
		case len(c.Profiles) == 0:
			c.Profiles = map[string]config.Profile{"default": {ItemID: ex.ItemID}}
			c.ActiveProfile = "default"
			profileAction = "created_default"
		case addUse:
			c.Profiles["default"] = config.Profile{ItemID: ex.ItemID}
			c.ActiveProfile = "default"
			profileAction = "repointed_default"
		}
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}

		out := map[string]string{
			"status":           "linked",
			"item_id":          ex.ItemID,
			"institution_id":   instID,
			"institution_name": instName,
			"env":              addEnv,
			"profile":          profileAction,
		}
		if profileAction == "unchanged" {
			out["hint"] = fmt.Sprintf("active profile %q still points at %q; pass --use to repoint it at this item, or run: fin profile save default --item %s", c.ActiveProfile, c.Profiles[c.ActiveProfile].ItemID, ex.ItemID)
		}
		return output.Emit(out)
	},
}

// auth list shows linked institutions with redacted access tokens.

var authListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show linked institutions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		activeItem := ""
		if p, ok := c.Profiles[c.ActiveProfile]; ok {
			activeItem = p.ItemID
		}
		type row struct {
			ItemID        string `json:"item_id"`
			Provider      string `json:"provider"`
			Env           string `json:"env"`
			Institution   string `json:"institution_name"`
			InstitutionID string `json:"institution_id"`
			AddedAt       string `json:"added_at"`
			TokenRedacted string `json:"token_redacted"`
			Active        bool   `json:"active"`
		}
		out := []row{}
		for id, it := range c.Items {
			tok, _ := config.GetSecret("plaid:item:" + id)
			red := "(missing)"
			if tok != "" && len(tok) > 8 {
				red = tok[:8] + "…" + tok[len(tok)-4:]
			}
			out = append(out, row{
				ItemID:        id,
				Provider:      it.Provider,
				Env:           it.Env,
				Institution:   it.InstitutionName,
				InstitutionID: it.InstitutionID,
				AddedAt:       it.AddedAt.Format(time.RFC3339),
				TokenRedacted: red,
				Active:        id == activeItem,
			})
		}
		return output.Emit(out)
	},
}

// auth remove deletes an item from config, cleans up the keychain, and prunes
// any profiles that pointed at it.

var authRemoveCmd = &cobra.Command{
	Use:   "remove <item-id>",
	Short: "Disconnect a linked institution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		id := args[0]
		if _, ok := c.Items[id]; !ok {
			return finerr.New(finerr.CodeNotFound, "item %q not found", id)
		}
		delete(c.Items, id)
		for name, p := range c.Profiles {
			if p.ItemID == id {
				delete(c.Profiles, name)
			}
		}
		if c.ActiveProfile != "" {
			if _, ok := c.Profiles[c.ActiveProfile]; !ok {
				c.ActiveProfile = ""
			}
		}
		_ = config.DeleteSecret("plaid:item:" + id)
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}
		return output.Emit(map[string]string{"status": "removed", "item_id": id})
	},
}

func init() {
	authSetupCmd.Flags().StringVar(&setupClientID, "client-id", "", "Plaid client_id")
	authSetupCmd.Flags().StringVar(&setupSecret, "secret", "", "Plaid secret")
	authSetupCmd.Flags().StringVar(&setupEnv, "env", "sandbox", "sandbox or production")
	authCmd.AddCommand(authSetupCmd)

	authAddCmd.Flags().StringVar(&addEnv, "env", "", "sandbox or production (defaults to value from `fin auth setup`)")
	authAddCmd.Flags().StringVar(&addPublicTok, "public-token", "", "skip browser flow; exchange this public_token directly")
	authAddCmd.Flags().BoolVar(&addUse, "use", false, "point the `default` profile at this newly-linked item and activate it")
	authCmd.AddCommand(authAddCmd)

	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authRemoveCmd)
}
