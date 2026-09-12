package xfkey

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type backupCheckingTransport struct {
	*settingsTransport
	dir     string
	checked bool
}

func (f *backupCheckingTransport) StartWrite(b []byte, deadline time.Time) (<-chan writeResult, error) {
	if b[2] == 2 {
		if _, _, err := loadCapture(f.dir); err != nil {
			return nil, err
		}
		for _, name := range []string{"backup.json", "plan.json", "intended-configuration.bin"} {
			if _, err := os.ReadFile(filepath.Join(f.dir, name)); err != nil {
				return nil, err
			}
		}
		f.checked = true
	}
	return f.settingsTransport.StartWrite(b, deadline)
}

func TestSaveConfirmation(t *testing.T) {
	for _, value := range []string{"n\n", "N\n", "no\n", "NO\n", "write\n", "Write\n", " y\n", "yes \n", " \n", "invalid\n"} {
		if saveConfirmation(value) {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, value := range []string{"\n", "\r\n", "y\n", "Y\n", "yes\n", "YES\n", "Yes\r\n"} {
		if !saveConfirmation(value) {
			t.Fatalf("rejected %q", value)
		}
	}
}
