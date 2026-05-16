package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// CLIError is the structured error written to stderr on non-zero exits.
// Implements ExitCoder (interface defined in internal/cmd).
type CLIError struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Hint        string   `json:"hint,omitempty"`
	ValidValues []string `json:"valid_values,omitempty"`
	Exit        int      `json:"exit_code"`
}

func (e *CLIError) Error() string { return e.Message }
func (e *CLIError) ExitCode() int { return e.Exit }

// Errorf builds a CLIError, writes the structured envelope to stderr, and returns
// it. The caller returns the error from RunE; main.go reads ExitCode().
func Errorf(exit int, code, format string, args ...any) error {
	e := &CLIError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Exit:    exit,
	}
	emitStderr(e)
	return e
}

// ErrorfHint is Errorf with an explicit `hint` field set.
func ErrorfHint(exit int, code, hint, format string, args ...any) error {
	e := &CLIError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Hint:    hint,
		Exit:    exit,
	}
	emitStderr(e)
	return e
}

// ErrorfEnum is Errorf with a populated `valid_values` field — for enum rejections.
// Lets agents self-correct in one step.
func ErrorfEnum(exit int, code string, valid []string, format string, args ...any) error {
	e := &CLIError{
		Code:        code,
		Message:     fmt.Sprintf(format, args...),
		ValidValues: valid,
		Exit:        exit,
	}
	emitStderr(e)
	return e
}

func emitStderr(e *CLIError) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"error": e})
}
