package errors

import "fmt"

const (
	CodeOK          = 0
	CodeGeneric     = 1
	CodeUsage       = 2
	CodeNotFound    = 3
	CodeAuth        = 4
	CodeUpstream    = 5
	CodeConflict    = 6
	CodeRateLimited = 7
	CodeNetwork     = 8
	CodeValidation  = 9
	CodeTimeout     = 124
)

type ExitError struct {
	code int
	msg  string
	err  error
}

func (e *ExitError) Error() string { return e.msg }
func (e *ExitError) Code() int     { return e.code }
func (e *ExitError) Unwrap() error { return e.err }

func Wrap(err error, code int, format string, args ...any) *ExitError {
	return &ExitError{code: code, msg: fmt.Sprintf(format, args...), err: err}
}

func New(code int, format string, args ...any) *ExitError {
	return &ExitError{code: code, msg: fmt.Sprintf(format, args...)}
}
