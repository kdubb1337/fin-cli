package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/output"
	"github.com/kdubb1337/fin-cli/internal/store"
)

var (
	sqlStmt    string
	sqlMaxRows int
)

var sqlCmd = &cobra.Command{
	Use:   "sql [-- <statement>]",
	Short: "Run a read-only SQL query against the local cache",
	Long: `Opens ~/.fin/cache.db (override with $FIN_HOME) in read-only mode and
executes one SELECT/WITH/EXPLAIN/PRAGMA statement, emitting JSON rows on stdout.

The statement can come from:
  - --query "<sql>"
  - the first positional argument
  - stdin (when neither is given), terminated by EOF

Only read-only statements are accepted: the DB handle is opened with mode=ro
and query_only=1 at the SQLite layer, and the parser additionally rejects any
statement that doesn't start with SELECT, WITH, EXPLAIN, or PRAGMA.`,
	Example: `  fin sql "SELECT date, name, amount FROM transactions ORDER BY date DESC LIMIT 10"
  echo "SELECT COUNT(*) AS n FROM transactions" | fin sql
  fin sql --query "SELECT * FROM accounts" --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stmt := strings.TrimSpace(sqlStmt)
		if stmt == "" && len(args) == 1 {
			stmt = strings.TrimSpace(args[0])
		}
		if stmt == "" {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return finerr.Wrap(err, finerr.CodeUsage, "read stdin: %v", err)
			}
			stmt = strings.TrimSpace(string(raw))
		}
		if stmt == "" {
			return finerr.New(finerr.CodeUsage, "no SQL provided (use --query, a positional arg, or pipe via stdin)")
		}
		if err := assertReadOnlyStmt(stmt); err != nil {
			return err
		}

		s, err := store.OpenReadOnly("")
		if err != nil {
			if os.IsNotExist(err) {
				return finerr.New(finerr.CodeNotFound, "cache db not found; run `fin sync` first")
			}
			return finerr.Wrap(err, finerr.CodeGeneric, "open db: %v", err)
		}
		defer s.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := s.DB.QueryContext(ctx, stmt)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeValidation, "query: %v", err)
		}
		defer rows.Close()

		out, truncated, err := scanRows(rows, sqlMaxRows)
		if err != nil {
			return finerr.Wrap(err, finerr.CodeGeneric, "scan: %v", err)
		}
		if truncated {
			return output.EmitPage(out, "", fmt.Sprintf("truncated at --max-rows=%d", sqlMaxRows))
		}
		return output.Emit(out)
	},
}

// assertReadOnlyStmt is belt to the read-only handle's suspenders. SQLite
// already refuses writes via mode=ro+query_only, but a clear error here turns
// "attempt to write a readonly database" into a Code-2 usage error so the
// agent doesn't retry with the same query.
func assertReadOnlyStmt(stmt string) error {
	cleaned := stripSQLComments(stmt)
	if cleaned == "" {
		return finerr.New(finerr.CodeUsage, "statement is empty after comment stripping")
	}
	// Reject anything with a semicolon followed by more SQL — multi-statement
	// scripts are forbidden so a malicious payload can't hide a write after a
	// safe SELECT. A trailing semicolon is fine.
	if i := strings.IndexByte(cleaned, ';'); i >= 0 && strings.TrimSpace(cleaned[i+1:]) != "" {
		return finerr.New(finerr.CodeUsage, "only one statement per call is allowed")
	}
	first := strings.ToUpper(strings.Fields(cleaned)[0])
	switch first {
	case "SELECT", "WITH", "EXPLAIN", "PRAGMA":
		return nil
	}
	return finerr.New(finerr.CodeUsage,
		"only SELECT/WITH/EXPLAIN/PRAGMA statements are allowed in `fin sql` (got %q)", first)
}

// stripSQLComments removes "-- line" and "/* block */" comments so the parser
// sees the leading keyword even if the agent prefixes the query with notes.
func stripSQLComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch {
		case i+1 < len(s) && s[i] == '-' && s[i+1] == '-':
			// line comment
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				i = len(s)
			} else {
				i += j + 1
				b.WriteByte('\n')
			}
		case i+1 < len(s) && s[i] == '/' && s[i+1] == '*':
			j := strings.Index(s[i:], "*/")
			if j < 0 {
				i = len(s)
			} else {
				i += j + 2
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return strings.TrimSpace(b.String())
}

func scanRows(rows *sql.Rows, maxRows int) ([]map[string]any, bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}
	var out []map[string]any
	truncated := false
	for rows.Next() {
		if maxRows > 0 && len(out) >= maxRows {
			truncated = true
			break
		}
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, false, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalize(dest[i])
		}
		out = append(out, row)
	}
	return out, truncated, rows.Err()
}

// normalize converts SQLite's BLOB-as-[]byte default into strings when the
// content is valid UTF-8 — agents almost always want the text shape.
func normalize(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func init() {
	sqlCmd.Flags().StringVar(&sqlStmt, "query", "", "SQL to run (alternative to positional arg / stdin)")
	sqlCmd.Flags().IntVar(&sqlMaxRows, "max-rows", 1000, "cap result rows to avoid OOM (0 = unbounded)")
	rootCmd.AddCommand(sqlCmd)
}
