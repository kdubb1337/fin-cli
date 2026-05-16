package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kdubb1337/fin-cli/internal/config"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	plaidprov "github.com/kdubb1337/fin-cli/internal/provider/plaid"
)

// This file holds minimum-viable stubs for the four "Rung 3 floor" commands:
//   - doctor          : health check across config + creds + API
//   - agent-context   : versioned structured introspection
//   - profile         : save / use / list / show / delete
//   - auth            : add / list / remove
//
// Each is wired into the root command and produces the right output shape.
// Replace the bodies with real implementations as you build.

// --- doctor -----------------------------------------------------------------

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Health check: config, credentials, API reachability",
	Example: `  fin doctor --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		type check struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail,omitempty"`
		}
		out := []check{}

		c, err := config.Load()
		out = append(out, check{Name: "config_load", OK: err == nil, Detail: errStr(err)})
		if err != nil {
			_ = output.Emit(out)
			return finerr.New(finerr.CodeGeneric, "config load failed")
		}

		out = append(out, check{Name: "plaid_client_id_set", OK: c.Plaid.ClientID != ""})
		out = append(out, check{
			Name:   "plaid_env_valid",
			OK:     c.Plaid.Env == "sandbox" || c.Plaid.Env == "production",
			Detail: c.Plaid.Env,
		})

		_, secErr := config.GetSecret("plaid:client_secret")
		out = append(out, check{Name: "plaid_secret_in_keychain", OK: secErr == nil, Detail: errStr(secErr)})

		out = append(out, check{
			Name:   "linked_item_count",
			OK:     true,
			Detail: fmt.Sprintf("%d / 10 (Plaid Trial cap)", len(c.Items)),
		})
		if len(c.Items) > 8 {
			out = append(out, check{
				Name:   "trial_cap_warning",
				OK:     false,
				Detail: "approaching Plaid Trial 10-item cap",
			})
		}

		if c.Plaid.ClientID != "" && secErr == nil && len(c.Items) > 0 {
			client, cerr := plaidprov.New(c)
			if cerr == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				for id, it := range c.Items {
					tok, _ := config.GetSecret("plaid:item:" + id)
					herr := client.Health(ctx, tok)
					out = append(out, check{
						Name:   "item_health:" + id,
						OK:     herr == nil,
						Detail: it.InstitutionName + " — " + errStr(herr),
					})
				}
			}
		}

		if err := output.Emit(out); err != nil {
			return err
		}
		for _, c := range out {
			if !c.OK {
				return finerr.New(finerr.CodeGeneric, "one or more checks failed")
			}
		}
		return nil
	},
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// --- agent-context ----------------------------------------------------------

// SchemaVersion is bumped on any breaking change to the agent-context output shape.
const SchemaVersion = 1

var agentContextCmd = &cobra.Command{
	Use:   "agent-context",
	Short: "Emit versioned structured introspection for AI agents",
	Long: `Emits a JSON document describing all commands, flags, enums, profiles,
and exit codes. Agents read this once instead of crawling --help.

Bumps schema_version on breaking shape changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := buildAgentContext(rootCmd)
		out, err := json.MarshalIndent(ctx, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(out))
		return nil
	},
}

func buildAgentContext(root *cobra.Command) map[string]any {
	return map[string]any{
		"schema_version": SchemaVersion,
		"cli":            root.Name(),
		"version":        version,
		"exit_codes": map[string]int{
			"ok": 0, "generic": 1, "usage": 2, "not_found": 3, "auth": 4,
			"api": 5, "conflict": 6, "rate_limit": 7, "network": 8,
			"validation": 9, "timeout": 124,
		},
		"commands": describeCommands(root),
	}
}

func describeCommands(c *cobra.Command) []map[string]any {
	out := make([]map[string]any, 0, len(c.Commands()))
	for _, sub := range c.Commands() {
		if sub.Hidden || !sub.IsAvailableCommand() {
			continue
		}
		flags := []map[string]any{}
		sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
			flags = append(flags, map[string]any{
				"name":    f.Name,
				"type":    f.Value.Type(),
				"default": f.DefValue,
				"usage":   f.Usage,
			})
		})
		entry := map[string]any{
			"name":     sub.Name(),
			"use":      sub.Use,
			"short":    sub.Short,
			"example":  sub.Example,
			"flags":    flags,
			"children": describeCommands(sub),
		}
		out = append(out, entry)
	}
	return out
}

// --- profile ----------------------------------------------------------------
//
// Real implementation lives in profile.go.

// --- auth -------------------------------------------------------------------

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage credentials and accounts",
}

// authListCmd and authRemoveCmd live in auth.go (real impls).

// --- skill ------------------------------------------------------------------
//
// Manage the SKILL.md that ships inside this binary. Agents discover the CLI by
// dropping the bundled skill folder into their personal skills directory:
//
//   fin skill install claude codex   # symlink into ~/.claude/skills and ~/.codex/skills
//   fin skill install --all          # every agent whose parent dir exists
//   fin skill list                   # show install status across known agents
//   fin skill path                   # print the source SKILL.md path
//
// The cross-agent "universal" target (`agents`) writes to ~/.agents/skills/,
// which Gemini CLI and OpenHands V1 both honor as a tool-agnostic location.

// agentTarget describes one place a skill can be installed.
type agentTarget struct {
	Name  string // user-facing identifier (claude, codex, gemini, openhands, agents)
	Dir   string // absolute path to the agent's personal skills directory
	Notes string // short human description for `skill list` and help text
}

// agentRegistry returns the canonical list of known install targets.
// Keep ordered for stable output. Override paths via $FIN_SKILLS_<AGENT>.
func agentRegistry() []agentTarget {
	home, _ := os.UserHomeDir()
	expand := func(envKey, fallback string) string {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		return filepath.Join(home, fallback)
	}
	return []agentTarget{
		{Name: "claude", Dir: expand("FIN_SKILLS_CLAUDE", ".claude/skills"), Notes: "Claude Code (Anthropic)"},
		{Name: "codex", Dir: expand("FIN_SKILLS_CODEX", ".codex/skills"), Notes: "Codex CLI (OpenAI)"},
		{Name: "gemini", Dir: expand("FIN_SKILLS_GEMINI", ".gemini/skills"), Notes: "Gemini CLI (Google)"},
		{Name: "openhands", Dir: expand("FIN_SKILLS_OPENHANDS", ".openhands/microagents"), Notes: "OpenHands (V0 microagents path)"},
		{Name: "agents", Dir: expand("FIN_SKILLS_AGENTS", ".agents/skills"), Notes: "Cross-agent universal (Gemini, OpenHands V1)"},
	}
}

func agentNames() []string {
	reg := agentRegistry()
	names := make([]string, 0, len(reg))
	for _, a := range reg {
		names = append(names, a.Name)
	}
	return names
}

func lookupAgent(name string) (agentTarget, bool) {
	for _, a := range agentRegistry() {
		if a.Name == name {
			return a, true
		}
	}
	return agentTarget{}, false
}

// findSkillSource locates the directory containing the bundled SKILL.md.
// Returns the directory (e.g. .../skills/fin) and the SKILL.md path inside it.
func findSkillSource() (dir string, file string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	base := filepath.Dir(exe)
	// Homebrew layout: <prefix>/bin/<cli> + <prefix>/share/<cli>/skills/<cli>/SKILL.md
	// Dev / repo layout:  <repo>/bin/<cli> + <repo>/skills/<cli>/SKILL.md
	candidates := []string{
		filepath.Join(base, "..", "share", "fin", "skills", "fin"),
		filepath.Join(base, "skills", "fin"),
		filepath.Join(base, "..", "skills", "fin"),
	}
	for _, c := range candidates {
		p := filepath.Join(c, "SKILL.md")
		if _, statErr := os.Stat(p); statErr == nil {
			abs, _ := filepath.Abs(c)
			return abs, filepath.Join(abs, "SKILL.md"), nil
		}
	}
	return "", "", output.Errorf(1, "skill_not_found",
		"could not locate bundled SKILL.md; tried %v (os=%s)", candidates, runtime.GOOS)
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the bundled SKILL.md (path / install / uninstall / list)",
	Long: `Manage the SKILL.md that ships inside this binary so agents can discover
fin's verbs and conventions.

Known agent targets:
  claude     ~/.claude/skills        Claude Code (Anthropic)
  codex      ~/.codex/skills         Codex CLI (OpenAI)
  gemini     ~/.gemini/skills        Gemini CLI (Google)
  openhands  ~/.openhands/microagents OpenHands (V0)
  agents     ~/.agents/skills        Cross-agent universal path

Override any target's path with $FIN_SKILLS_<AGENT> (e.g.
FIN_SKILLS_CLAUDE=/opt/skills/claude).`,
}

// --- skill path -------------------------------------------------------------

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the absolute path to the bundled SKILL.md",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, file, err := findSkillSource()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, file)
		return nil
	},
}

// --- skill install ----------------------------------------------------------

var (
	flagSkillInstallMode string
	flagSkillInstallAll  bool
	flagSkillInstallDir  string
)

var skillInstallCmd = &cobra.Command{
	Use:   "install [agent...]",
	Short: "Install the bundled SKILL.md into one or more agent skills directories",
	Example: `  fin skill install claude
  fin skill install claude codex gemini
  fin skill install --all
  fin skill install --dir ~/.config/myagent/skills
  fin skill install claude --mode=copy --force --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillInstall(args, false)
	},
}

// --- skill uninstall --------------------------------------------------------

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall [agent...]",
	Short: "Remove the bundled SKILL.md from one or more agent skills directories",
	Example: `  fin skill uninstall claude
  fin skill uninstall --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillInstall(args, true)
	},
}

// runSkillInstall is the shared driver for install and uninstall.
func runSkillInstall(args []string, remove bool) error {
	if flagSkillInstallMode != "symlink" && flagSkillInstallMode != "copy" {
		return output.ErrorfEnum(2, "bad_mode",
			[]string{"symlink", "copy"},
			"--mode must be 'symlink' or 'copy' (got %q)", flagSkillInstallMode)
	}

	srcDir, _, err := findSkillSource()
	if err != nil {
		return err
	}

	targets, err := resolveTargets(args)
	if err != nil {
		return err
	}

	type result struct {
		Agent  string `json:"agent"`
		Path   string `json:"path"`
		Mode   string `json:"mode,omitempty"`
		Action string `json:"action"`
		Detail string `json:"detail,omitempty"`
	}
	results := make([]result, 0, len(targets))

	for _, t := range targets {
		dest := filepath.Join(t.Dir, "fin")
		r := result{Agent: t.Name, Path: dest, Mode: flagSkillInstallMode}

		if remove {
			r.Action, r.Detail, err = uninstallOne(dest)
		} else {
			r.Action, r.Detail, err = installOne(srcDir, dest, flagSkillInstallMode, flagForce, flagDryRun)
		}
		if err != nil {
			r.Action = "error"
			r.Detail = err.Error()
		}
		results = append(results, r)
	}

	verb := "install"
	if remove {
		verb = "uninstall"
	}
	payload := map[string]any{
		"action":  verb,
		"mode":    flagSkillInstallMode,
		"source":  srcDir,
		"dry_run": flagDryRun,
		"results": results,
	}
	if flagDryRun {
		return output.EmitDryRun(payload)
	}
	return output.Emit(payload)
}

// resolveTargets turns positional agent names + --all + --dir into a target list.
func resolveTargets(args []string) ([]agentTarget, error) {
	reg := agentRegistry()
	picked := make([]agentTarget, 0, len(reg)+len(args)+1)

	if flagSkillInstallAll {
		picked = append(picked, reg...)
	}

	for _, name := range args {
		a, ok := lookupAgent(name)
		if !ok {
			return nil, output.ErrorfEnum(2, "unknown_agent",
				agentNames(),
				"unknown agent %q", name)
		}
		picked = append(picked, a)
	}

	if flagSkillInstallDir != "" {
		expanded := flagSkillInstallDir
		if strings.HasPrefix(expanded, "~/") {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded[2:])
		}
		abs, _ := filepath.Abs(expanded)
		picked = append(picked, agentTarget{Name: "custom", Dir: abs, Notes: "user-specified --dir"})
	}

	if len(picked) == 0 {
		return nil, output.ErrorfEnum(2, "no_target",
			append(agentNames(), "--all", "--dir"),
			"no install target given; pass one or more of %v, or --all, or --dir <path>", agentNames())
	}

	// Deduplicate by destination path so `--all claude` doesn't double-install.
	seen := map[string]bool{}
	deduped := make([]agentTarget, 0, len(picked))
	for _, t := range picked {
		if seen[t.Dir] {
			continue
		}
		seen[t.Dir] = true
		deduped = append(deduped, t)
	}
	sort.SliceStable(deduped, func(i, j int) bool { return deduped[i].Name < deduped[j].Name })
	return deduped, nil
}

// installOne creates or refreshes <dest> pointing at <srcDir>.
// Returns (action, detail, err). action is one of: created, refreshed, skipped, would-create, would-refresh.
func installOne(srcDir, dest, mode string, force, dryRun bool) (string, string, error) {
	parent := filepath.Dir(dest)

	// Inspect any existing destination.
	existing, statErr := os.Lstat(dest)
	if statErr != nil && !os.IsNotExist(statErr) {
		return "error", "", statErr
	}

	// Idempotent fast-path: existing symlink already points at our source.
	if existing != nil && existing.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(dest)
		resolved, _ := filepath.Abs(target)
		if resolved == srcDir && mode == "symlink" {
			return "skipped", "symlink already points at source", nil
		}
	}

	if existing != nil && !force {
		return "skipped", fmt.Sprintf("destination exists; pass --force to overwrite (%s)", dest), nil
	}

	action, intent := "created", "create"
	if existing != nil {
		action, intent = "refreshed", "refresh"
	}
	if dryRun {
		return "would-" + intent, fmt.Sprintf("%s → %s (mode=%s)", srcDir, dest, mode), nil
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "error", "", err
	}
	if existing != nil {
		if err := os.RemoveAll(dest); err != nil {
			return "error", "", err
		}
	}

	switch mode {
	case "symlink":
		if err := os.Symlink(srcDir, dest); err != nil {
			return "error", "", err
		}
	case "copy":
		if err := copyDir(srcDir, dest); err != nil {
			return "error", "", err
		}
	}
	return action, fmt.Sprintf("%s → %s", srcDir, dest), nil
}

// uninstallOne removes <dest> if it's a symlink to our skill or a regular dir.
func uninstallOne(dest string) (string, string, error) {
	info, err := os.Lstat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return "skipped", "not installed", nil
		}
		return "error", "", err
	}
	if flagDryRun {
		kind := "directory"
		if info.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
		}
		return "would-remove", fmt.Sprintf("remove %s at %s", kind, dest), nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return "error", "", err
	}
	return "removed", dest, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// --- skill list -------------------------------------------------------------

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the bundled SKILL.md install status across known agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir, _, err := findSkillSource()
		if err != nil {
			return err
		}
		type row struct {
			Agent     string `json:"agent"`
			Path      string `json:"path"`
			Notes     string `json:"notes"`
			Installed bool   `json:"installed"`
			Mode      string `json:"mode,omitempty"`
			LinksToUs bool   `json:"links_to_us,omitempty"`
		}
		var rows []row
		for _, t := range agentRegistry() {
			dest := filepath.Join(t.Dir, "fin")
			r := row{Agent: t.Name, Path: dest, Notes: t.Notes}
			if info, err := os.Lstat(dest); err == nil {
				r.Installed = true
				if info.Mode()&os.ModeSymlink != 0 {
					r.Mode = "symlink"
					if target, err := os.Readlink(dest); err == nil {
						resolved, _ := filepath.Abs(target)
						r.LinksToUs = resolved == srcDir
					}
				} else if info.IsDir() {
					r.Mode = "copy"
				} else {
					r.Mode = "file"
				}
			}
			rows = append(rows, r)
		}
		return output.Emit(map[string]any{
			"source":  srcDir,
			"targets": rows,
		})
	},
}

func init() {
	skillInstallCmd.Flags().StringVar(&flagSkillInstallMode, "mode", "symlink", "install mode: symlink|copy")
	skillInstallCmd.Flags().BoolVar(&flagSkillInstallAll, "all", false, "install to every known agent in the registry")
	skillInstallCmd.Flags().StringVar(&flagSkillInstallDir, "dir", "", "additional custom skills directory to install into")
	skillUninstallCmd.Flags().BoolVar(&flagSkillInstallAll, "all", false, "uninstall from every known agent in the registry")
	skillUninstallCmd.Flags().StringVar(&flagSkillInstallDir, "dir", "", "additional custom skills directory to uninstall from")
	// uninstall reuses runSkillInstall but ignores --mode; the shared default
	// of "symlink" (set by install's flag registration above) satisfies the
	// mode validation without exposing the flag here.

	skillCmd.AddCommand(skillPathCmd, skillInstallCmd, skillUninstallCmd, skillListCmd)
	rootCmd.AddCommand(doctorCmd, agentContextCmd, authCmd, skillCmd)
}
