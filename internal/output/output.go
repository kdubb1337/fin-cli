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
	"unicode"

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
		widths[i] = displayWidth(h)
	}
	body := make([][]string, len(rows))
	for ri, r := range rows {
		cells := make([]string, len(headers))
		for i, h := range headers {
			cells[i] = formatCell(r[h])
			if n := displayWidth(cells[i]); n > widths[i] {
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
		pad := widths[i] - displayWidth(c)
		if pad < 0 {
			pad = 0
		}
		parts[i] = c + strings.Repeat(" ", pad)
	}
	fmt.Fprintln(w, strings.Join(parts, "  "))
}

// displayWidth approximates the terminal cell width of s. Combining marks,
// variation selectors, and ZWJ contribute 0; CJK and default-emoji-presentation
// code points contribute 2; everything else contributes 1. Good enough for
// table alignment without pulling in golang.org/x/text or go-runewidth.
//
// Note: in the U+2600-U+27BF block most symbols (☠ ★ ✓ ☂) are "default text
// presentation" per UTS #51 and render 1-wide in most terminals even when
// followed by VS-16. Only specific Emoji_Presentation=Yes code points are
// promoted to wide here.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r == 0x200D, // zero-width joiner
			r >= 0xFE00 && r <= 0xFE0F, // variation selectors
			r == 0x2060, r == 0xFEFF,
			unicode.IsMark(r):
			// zero width
		case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
			r >= 0x2E80 && r <= 0x303E, // CJK radicals/punctuation
			r >= 0x3041 && r <= 0x33FF, // Hiragana/Katakana/CJK symbols
			r >= 0x3400 && r <= 0x4DBF, // CJK Ext A
			r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
			r >= 0xA000 && r <= 0xA4CF, // Yi
			r >= 0xAC00 && r <= 0xD7A3, // Hangul Syllables
			r >= 0xF900 && r <= 0xFAFF, // CJK Compat
			r >= 0xFE30 && r <= 0xFE4F, // CJK Compat Forms
			r >= 0xFF00 && r <= 0xFF60, // Fullwidth
			r >= 0xFFE0 && r <= 0xFFE6,
			r >= 0x1F300 && r <= 0x1F64F, // Emoji
			r >= 0x1F680 && r <= 0x1F6FF, // Transport
			r >= 0x1F700 && r <= 0x1F9FF, // Supplemental symbols
			r >= 0x1FA00 && r <= 0x1FAFF, // Symbols Ext-A
			r >= 0x20000 && r <= 0x3FFFD, // CJK Ext B-F
			isDefaultEmojiPresentation(r):
			w += 2
		default:
			w++
		}
	}
	return w
}

// isDefaultEmojiPresentation reports whether r is in the BMP and has the
// Unicode property Emoji_Presentation=Yes (UTS #51), i.e. it renders as a
// wide emoji glyph even without a VS-16 selector. Covers the singletons in
// the U+2300-U+27BF symbol blocks plus a few in U+2B00-U+2BFF.
func isDefaultEmojiPresentation(r rune) bool {
	switch r {
	case 0x231A, 0x231B, // watch, hourglass
		0x23F0, 0x23F3, // alarm, hourglass flowing
		0x25FD, 0x25FE, // small squares
		0x2614, 0x2615, // umbrella, hot beverage
		0x267F,         // wheelchair
		0x2693,         // anchor
		0x26A1,         // high voltage
		0x26AA, 0x26AB, // circles
		0x26BD, 0x26BE, // soccer, baseball
		0x26C4, 0x26C5, // snowman, sun behind cloud
		0x26CE,         // ophiuchus
		0x26D4,         // no entry
		0x26EA,         // church
		0x26F5,         // boat
		0x26FA,         // tent
		0x26FD,         // fuel pump
		0x2705,         // white heavy check
		0x270A, 0x270B, // fist, raised hand
		0x2728,         // sparkles
		0x274C, 0x274E, // cross marks
		0x2757,         // heavy exclamation
		0x27B0, 0x27BF, // curly loops
		0x2B50, 0x2B55: // star, hollow red circle
		return true
	}
	switch {
	case r >= 0x23E9 && r <= 0x23EC, // fast-forward/rewind
		r >= 0x2648 && r <= 0x2653, // zodiac
		r >= 0x26F2 && r <= 0x26F3, // fountain, golf
		r >= 0x2753 && r <= 0x2755, // question marks
		r >= 0x2795 && r <= 0x2797: // heavy plus/minus/divide
		return true
	}
	return false
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
