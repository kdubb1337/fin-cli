package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kdubb1337/fin-cli/internal/output"
)

// hello is a sample resource demonstrating the full pattern:
//   - <cli> hello get <name>            : single-resource read with --json/--compact/--select
//   - <cli> hello list                  : bounded list with --limit/--cursor
//   - <cli> hello create <name>         : mutation with --dry-run and structured response
//
// Delete or replace with your real resource once the patterns are clear.

type helloResource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Greeting  string `json:"greeting"`
	CreatedAt string `json:"created_at"`
}

var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Sample resource — replace with your real one",
}

var helloGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get a greeting by name",
	Args:  cobra.ExactArgs(1),
	Example: `  # JSON for an agent
  fin hello get world --json

  # Just the high-gravity fields
  fin hello get world --compact`,
	RunE: func(cmd *cobra.Command, args []string) error {
		res := helloResource{
			ID:        "hello-1",
			Name:      args[0],
			Greeting:  "Hello, " + args[0] + "!",
			CreatedAt: "2026-05-13T00:00:00Z",
		}
		return output.Emit(res)
	},
}

var (
	helloListLimit  int
	helloListCursor string
)

var helloListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List greetings",
	Example: `  fin hello list --limit 5 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		items := []helloResource{
			{ID: "hello-1", Name: "world", Greeting: "Hello, world!"},
			{ID: "hello-2", Name: "agent", Greeting: "Hello, agent!"},
		}
		if helloListLimit > 0 && helloListLimit < len(items) {
			return output.EmitPage(items[:helloListLimit], "next-page-cursor", "more available; pass --cursor=<value>")
		}
		return output.Emit(items)
	},
}

var helloCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new greeting",
	Args:  cobra.ExactArgs(1),
	Example: `  fin hello create world --dry-run
  fin hello create world`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDryRun {
			return output.EmitDryRun(map[string]any{
				"would_create": helloResource{Name: args[0]},
			})
		}
		res := helloResource{
			ID:        "hello-new",
			Name:      args[0],
			Greeting:  "Hello, " + args[0] + "!",
			CreatedAt: "2026-05-13T00:00:00Z",
		}
		return output.Emit(res)
	},
}

func init() {
	helloListCmd.Flags().IntVar(&helloListLimit, "limit", 25, "max items to return")
	helloListCmd.Flags().StringVar(&helloListCursor, "cursor", "", "pagination cursor from previous response")

	helloCmd.AddCommand(helloGetCmd, helloListCmd, helloCreateCmd)
	rootCmd.AddCommand(helloCmd)
}
