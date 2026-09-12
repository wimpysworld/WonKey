package xfkey

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

var errWriteRequired = errors.New("settings write requires explicit authorisation; no device access")

type cliRuntime struct {
	in          io.Reader
	out, errOut io.Writer
}

type CLIError struct {
	Err  error
	Code int
}

func (e *CLIError) Error() string { return e.Err.Error() }
func (e *CLIError) Unwrap() error { return e.Err }

func ExitCode(err error) int {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return 1
}

func readerInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func saveConfirmation(line string) bool {
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line == "" || strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")
}
