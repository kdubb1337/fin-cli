package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"
)

// Discipline:
//   - stdout = data; stderr = humans + diagnostics.
//   - Auto-JSON when stdout is not a TTY. --human forces tables in a pipe.
//   - Only one of --json / --human / --csv may be set; validated by Cobra's MarkFlagsMutuallyExclusive.

var (
	cfg Options

	out = os.Stdout
	err = os.Stderr
)

// Options carries the flag state from root.go to output helpers.
type Options struct {
	JSON, Human, CSV      bool
	Compact               bool
	Select                string
	Quiet, Verbose, Debug bool
}

// Configure is called from PersistentPreRunE.
func Configure(o Options) {
	cfg = o
}

// mode returns the resolved output mode after auto-detection.
func mode() string {
	switch {
	case cfg.JSON:
		return "json"
	case cfg.Human:
		return "human"
	case cfg.CSV:
		return "csv"
	default:
		if isatty.IsTerminal(os.Stdout.Fd()) {
			return "human"
		}
		return "json"
	}
}

// Emit renders a single data payload to stdout in the resolved mode.
func Emit(v any) error {
	v = project(v)
	switch mode() {
	case "json":
		return emitJSON(map[string]any{"data": v})
	case "csv":
		return emitCSV(v)
	default:
		return emitHuman(v)
	}
}

// EmitPage renders a paginated list with truncation metadata.
func EmitPage(items any, nextCursor, hint string) error {
	items = project(items)
	envelope := map[string]any{
		"data": items,
		"pagination": map[string]any{
			"next_cursor": nextCursor,
			"has_more":    nextCursor != "",
		},
		"truncated": nextCursor != "",
	}
	if hint != "" {
		envelope["hint"] = hint
	}
	switch mode() {
	case "json":
		return emitJSON(envelope)
	case "csv":
		return emitCSV(items)
	default:
		if hint != "" {
			Progress("note: %s", hint)
		}
		return emitHuman(items)
	}
}

// EmitDryRun renders a payload tagged as a dry-run.
func EmitDryRun(v any) error {
	wrapped := map[string]any{
		"dry_run": true,
	}
	if m, ok := v.(map[string]any); ok {
		for k, vv := range m {
			wrapped[k] = vv
		}
	} else {
		wrapped["data"] = v
	}
	return emitJSON(wrapped)
}

// --- helpers ----------------------------------------------------------------

func emitJSON(v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func emitCSV(v any) error {
	rows := toRows(v)
	if len(rows) == 0 {
		return nil
	}
	w := csv.NewWriter(out)
	defer w.Flush()
	header := orderedKeys(unionKeys(rows))
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		rec := make([]string, len(header))
		for i, k := range header {
			rec[i] = formatCell(r[k])
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return nil
}

// emitHuman renders a plain-text table to stdout. Columns are stable across
// runs (priority list + alpha fallback). Slices become row-per-item tables;
// single objects render as a one-row table.
func emitHuman(v any) error {
	rows := toRows(v)
	if len(rows) == 0 {
		fmt.Fprintln(out, "(no rows)")
		return nil
	}
	headers := orderedKeys(unionKeys(rows))
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	body := make([][]string, len(rows))
	for ri, r := range rows {
		cells := make([]string, len(headers))
		for i, h := range headers {
			cells[i] = formatCell(r[h])
			if n := len(cells[i]); n > widths[i] {
				widths[i] = n
			}
		}
		body[ri] = cells
	}
	writeHumanRow(out, headers, widths)
	seps := make([]string, len(headers))
	for i, w := range widths {
		seps[i] = strings.Repeat("-", w)
	}
	writeHumanRow(out, seps, widths)
	for _, row := range body {
		writeHumanRow(out, row, widths)
	}
	return nil
}

func writeHumanRow(w io.Writer, cells []string, widths []int) {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = c + strings.Repeat(" ", widths[i]-len(c))
	}
	fmt.Fprintln(w, strings.Join(parts, "  "))
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%v", v)
}

func toRows(v any) []map[string]any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		rows := make([]map[string]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			rows[i] = toMap(rv.Index(i).Interface())
		}
		return rows
	}
	return []map[string]any{toMap(v)}
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

// humanFieldPriority gives CSV/human renderers a stable, semantically-ordered
// column layout. Fields present in a row land in this order first; everything
// else falls through to alphabetical. Tune freely — order here is the only
// place column priority is configured.
var humanFieldPriority = []string{
	"name", "institution_name",
	"date", "amount", "currency",
	"type", "subtype", "status", "active",
	"balance", "available_balance",
	"merchant_name", "pending",
	"added_at", "env", "provider",
	"mask", "official_name",
	"id", "item_id", "account_id", "institution_id", "token_redacted",
}

func unionKeys(rows []map[string]any) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			keys[k] = struct{}{}
		}
	}
	return keys
}

func orderedKeys(keys map[string]struct{}) []string {
	seen := map[string]bool{}
	order := make([]string, 0, len(keys))
	for _, k := range humanFieldPriority {
		if _, ok := keys[k]; ok {
			order = append(order, k)
			seen[k] = true
		}
	}
	rest := make([]string, 0, len(keys))
	for k := range keys {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(order, rest...)
}

// project applies --compact and --select to a payload before emission.
func project(v any) any {
	if !cfg.Compact && cfg.Select == "" {
		return v
	}
	var fields map[string]struct{}
	if cfg.Select != "" {
		fields = map[string]struct{}{}
		for _, f := range strings.Split(cfg.Select, ",") {
			fields[strings.TrimSpace(f)] = struct{}{}
		}
	}
	compactKeys := map[string]struct{}{
		"id": {}, "name": {}, "status": {}, "created_at": {}, "updated_at": {},
	}
	pick := func(m map[string]any) map[string]any {
		picked := map[string]any{}
		for k, vv := range m {
			if fields != nil {
				if _, ok := fields[k]; !ok {
					continue
				}
			} else if cfg.Compact {
				if _, ok := compactKeys[k]; !ok {
					continue
				}
			}
			picked[k] = vv
		}
		return picked
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]map[string]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = pick(toMap(rv.Index(i).Interface()))
		}
		return out
	}
	return pick(toMap(v))
}

// Progress writes a human-readable line to stderr (suppressed by --quiet).
func Progress(format string, args ...any) {
	if cfg.Quiet {
		return
	}
	fmt.Fprintf(err, format+"\n", args...)
}

// Verbose writes a verbose-level line to stderr (only when --verbose or --debug).
func Verbose(format string, args ...any) {
	if !cfg.Verbose && !cfg.Debug {
		return
	}
	fmt.Fprintf(err, format+"\n", args...)
}

// Debug writes a debug-level line to stderr (only when --debug).
func Debug(format string, args ...any) {
	if !cfg.Debug {
		return
	}
	fmt.Fprintf(err, "debug: "+format+"\n", args...)
}
