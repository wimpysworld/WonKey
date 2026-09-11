package xfkey

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(nil, &stdout, &stderr)
	if err == nil || !strings.HasPrefix(err.Error(), "usage: wonkey ") {
		t.Fatalf("usage error = %v, want wonkey program name", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("usage wrote output instead of returning an error")
	}
}

func TestRunHelp(t *testing.T) {
	for _, command := range []string{"inspect", "identify", "readback", "plan", "apply", "preview", "parse-identify", "parse-readback"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := Run([]string{command, "--help"}, &stdout, &stderr); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("help error = %v, want flag.ErrHelp", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("help wrote stdout: %q", stdout.String())
			}
			if !bytes.HasPrefix(stderr.Bytes(), []byte("Usage of wonkey "+command+":\n")) {
				t.Fatalf("help stderr = %q", stderr.String())
			}
		})
	}
}
