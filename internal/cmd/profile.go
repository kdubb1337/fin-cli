package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage named profiles (name -> item-id)",
	Example: `  fin profile list
  fin profile use prod                           # switch the active profile
  fin profile save default --item <item-id>     # repoint the default at a different item
  fin profile save prod    --item <item-id>     # create/update a named profile`,
}

var profileSaveItem string

var profileSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a profile mapping a name to an item-id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		if profileSaveItem == "" {
			return finerr.New(finerr.CodeUsage, "--item is required")
		}
		if _, ok := c.Items[profileSaveItem]; !ok {
			return finerr.New(finerr.CodeNotFound, "item %q not linked", profileSaveItem)
		}
		c.Profiles[args[0]] = config.Profile{ItemID: profileSaveItem}
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}
		return output.Emit(map[string]string{
			"status":  "saved",
			"profile": args[0],
			"item_id": profileSaveItem,
		})
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		if _, ok := c.Profiles[args[0]]; !ok {
			return finerr.New(finerr.CodeNotFound, "profile %q not found", args[0])
		}
		c.ActiveProfile = args[0]
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}
		return output.Emit(map[string]string{
			"status":         "ok",
			"active_profile": args[0],
		})
	},
}

var profileGetCmd = &cobra.Command{
	Use:   "get [<name>]",
	Short: "Show a profile (active if omitted)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		name := c.ActiveProfile
		if len(args) == 1 {
			name = args[0]
		}
		if name == "" {
			return finerr.New(finerr.CodeNotFound, "no active profile set")
		}
		p, ok := c.Profiles[name]
		if !ok {
			return finerr.New(finerr.CodeNotFound, "profile %q not found", name)
		}
		return output.Emit(map[string]any{
			"name":    name,
			"item_id": p.ItemID,
			"active":  name == c.ActiveProfile,
		})
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		type row struct {
			Name   string `json:"name"`
			ItemID string `json:"item_id"`
			Active bool   `json:"active"`
		}
		out := []row{}
		for name, p := range c.Profiles {
			out = append(out, row{Name: name, ItemID: p.ItemID, Active: name == c.ActiveProfile})
		}
		return output.Emit(out)
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "config: %v", err)
		}
		if _, ok := c.Profiles[args[0]]; !ok {
			return finerr.New(finerr.CodeNotFound, "profile %q not found", args[0])
		}
		delete(c.Profiles, args[0])
		if c.ActiveProfile == args[0] {
			c.ActiveProfile = ""
		}
		if err := config.Save(c); err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "save: %v", err)
		}
		return output.Emit(map[string]string{"status": "deleted", "profile": args[0]})
	},
}

func init() {
	profileSaveCmd.Flags().StringVar(&profileSaveItem, "item", "", "item-id to bind to this profile")
	profileCmd.AddCommand(profileSaveCmd, profileUseCmd, profileGetCmd, profileListCmd, profileDeleteCmd)
	rootCmd.AddCommand(profileCmd)
}
