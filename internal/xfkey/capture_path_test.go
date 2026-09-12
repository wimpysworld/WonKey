package xfkey

import (
	"os"
	"path/filepath"
	"testing"
)

func publicTestCaptureRoot(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	root := filepath.Join(state, "wonkey", "captures")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCaptureRootXDG(t *testing.T) {
	for _, tc := range []struct {
		name, xdg, home  string
		wantXDG, wantErr bool
	}{
		{"absolute", "absolute", "", true, false},
		{"unset", "unset", "absolute", false, false},
		{"empty", "", "absolute", false, false},
		{"relative", "relative/state", "absolute", false, false},
		{"literal-tilde", "~/state", "absolute", false, false},
		{"empty-home", "", "", false, true},
		{"unset-home", "", "unset", false, true},
		{"relative-home", "relative", "relative/home", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			xdg, home := tc.xdg, tc.home
			if xdg == "absolute" {
				xdg = filepath.Join(base, "state")
			}
			if home == "absolute" {
				home = filepath.Join(base, "home")
			}
			t.Setenv("XDG_STATE_HOME", xdg)
			t.Setenv("HOME", home)
			if tc.xdg == "unset" {
				if err := os.Unsetenv("XDG_STATE_HOME"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.home == "unset" {
				if err := os.Unsetenv("HOME"); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("WONKEY_CAPTURE_ROOT", filepath.Join(base, "retired-wonkey"))
			t.Setenv("XFKEY_CAPTURE_ROOT", filepath.Join(base, "retired-xfkey"))
			got, err := resolveCaptureRoot()
			if (err != nil) != tc.wantErr {
				t.Fatalf("root=%q error=%v", got, err)
			}
			if !tc.wantErr {
				want := filepath.Join(home, ".local", "state", "wonkey", "captures")
				if tc.wantXDG {
					want = filepath.Join(xdg, "wonkey", "captures")
				}
				if got != want {
					t.Fatalf("root=%q, want %q", got, want)
				}
			}
			entries, err := os.ReadDir(base)
			if err != nil || len(entries) != 0 {
				t.Fatalf("resolver created storage: %v, %v", entries, err)
			}
		})
	}
}
