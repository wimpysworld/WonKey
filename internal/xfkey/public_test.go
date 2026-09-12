package xfkey

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPublicParser(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want Changes
	}{
		{[]string{"key"}, nil},
		{[]string{"rgb"}, nil},
		{[]string{"key", "f13"}, Changes{"key": 0x68, "modifiers": 0}},
		{[]string{"key", "ctrl+shift+f13"}, Changes{"key": 0x68, "modifiers": 3}},
		{[]string{"key", "alt+super+enter", "--on", "release"}, Changes{"key": 0x28, "modifiers": 12, "trigger": 2}},
		{[]string{"key", "SUPER+f13"}, Changes{"key": 0x68, "modifiers": 8}},
		{[]string{"key", "--on=both", "f13"}, Changes{"key": 0x68, "modifiers": 0, "trigger": 3}},
		{[]string{"key", "f13", "--on", "press"}, Changes{"key": 0x68, "modifiers": 0, "trigger": 1}},
		{[]string{"rgb", "steady", "0000ff"}, Changes{"rgb-mode": 1, "red": 0, "green": 0, "blue": 255}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := parsePublic(tc.args)
			if err != nil || !reflect.DeepEqual(got.changes, tc.want) {
				t.Fatal(got, err)
			}
		})
	}
	for name, mode := range lightingValues {
		c, err := parsePublic([]string{"rgb", name})
		if err != nil || !reflect.DeepEqual(c.changes, Changes{"rgb-mode": mode}) {
			t.Fatal(c, err)
		}
		original := newSettingsTransport().current
		changed, err := changeConfiguration(original, c.changes)
		if err != nil {
			t.Fatal(err)
		}
		for i := range original {
			if i != 124 && original[i] != changed[i] {
				t.Fatalf("%s changed byte %d", name, i)
			}
		}
	}
	for _, expression := range []string{"f13", "shift+f13", "super+f13", "ctrl+shift+alt+super+enter"} {
		c, err := parsePublic([]string{"key", expression})
		if err != nil {
			t.Fatal(err)
		}
		original := newSettingsTransport().current
		original[2], original[1] = 15, 2
		changed, err := changeConfiguration(original, c.changes)
		if err != nil || int(changed[2]) != c.changes["modifiers"] || changed[1] != 2 {
			t.Fatal(changed, err)
		}
		for i := range original {
			if i != 2 && i != 4 && original[i] != changed[i] {
				t.Fatal("unrelated byte changed", i)
			}
		}
	}
}

func TestPublicRefusesInvalidBeforeAccess(t *testing.T) {
	cases := [][]string{
		{"key", "gui+f13"},
		{"key", "GUI+enter"},
		{"key", "super+super+f13"},
		{"restore", "one", "two"},
		{"restore", "--yes"},
		{"restore", "--capture-root", "/tmp"},
		{"restore", "--write"},
		{"restore", "--help", "--yes"},
		{"rollback"},
		{"help"},
		{"advanced"},
		{"show"},
		{"set", "key=f13"},
		{"plan"},
		{"apply"},
		{"devices"},
		{"inspect"},
		{"identify"},
		{"readback"},
		{"preview"},
		{"protocol"},
		{"parse-identify"},
		{"parse-readback"},
		{"key", "--on", "release"},
		{"key", "f13", "--on=press", "--on=release"},
		{"key", "f13", "--on="},
		{"key", "f13", "--on", "click"},
		{"key", "f13", "--on"},
		{"rgb", "off", "--on", "press"},
		{"key", "a"},
		{"key", ""},
		{"key", "f13+"},
		{"key", "+f13"},
		{"key", "ctrl+ctrl+f13"},
		{"key", "none+f13"},
		{"key", "f13+ctrl"},
		{"key", "f13", "enter"},
		{"rgb", ""},
		{"rgb", "blue"},
		{"rgb", "steady", "#0000ff"},
		{"rgb", "steady", "00000"},
		{"rgb", "steady", "gggggg"},
		{"rgb", "off", "000000", "extra"},
		{"--help", "--yes"},
	}
	for _, flag := range []string{"--device", "--capture-root", "--json", "--yes", "--dry-run", "--write", "--target", "--path", "--key", "--modifiers", "--trigger", "--lighting", "--colour", "--rgb-mode", "--red", "--expect-identifier", "--expect-version", "--json=false", "--yes=false"} {
		cases = append(cases, []string{"key", "f13", flag}, []string{"rgb", "off", flag}, []string{"key", "--help", flag})
	}
	access := publicAccess{discover: func() ([]Candidate, error) { t.Fatal("discovery on invalid input"); return nil, nil }, interactive: func(io.Reader) bool { t.Fatal("runtime on invalid input"); return false }}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			err := runPublic(args, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
			if err == nil || ExitCode(err) != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
				t.Fatal(err, out.String(), diagnostic.String())
			}
		})
	}
}

func TestPublicHelpNoAccess(t *testing.T) {
	access := publicAccess{interactive: func(io.Reader) bool { t.Fatal("help accessed runtime"); return false }}
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"key", "--help"}, {"rgb", "--help"}, {"restore", "--help"}, {"key", "f13", "--help"}, {"rgb", "off", "-h"}} {
		var out bytes.Buffer
		if err := runPublic(args, &cliRuntime{strings.NewReader(""), &out, io.Discard}, access); err != nil {
			t.Fatal(err)
		}
		for _, removed := range []string{"--json", "--yes", "--device", "--dry-run", "advanced", "wonkey set", "Apply result"} {
			if strings.Contains(out.String(), removed) {
				t.Fatal(removed, out.String())
			}
		}
		if !strings.Contains(out.String(), "Usage:") || strings.Contains(out.String(), "\x1b") {
			t.Fatal(out.String())
		}
	}
}

func TestPublicWorkflow(t *testing.T) {
	for _, name := range []string{"confirm", "confirm-blank", "confirm-upper", "confirm-yes", "confirm-yes-upper", "cancel-upper", "cancel-no", "cancel-no-upper", "old-confirm", "space-only", "rgb-write", "clear-modifiers", "pasted-crlf", "pasted-selection", "second-device", "query-key", "query-rgb", "query-multiple", "zero", "incompatible", "cancel", "eof", "partial-write", "invalid-confirm", "space-confirm", "blank-selection", "eof-selection", "partial-selection", "invalid-selection", "out-of-range", "no-op", "noninteractive", "noninteractive-selection", "identity-changed", "version-changed", "settings-changed", "became-no-op", "mismatch", "echo", "deadline", "unsupported", "backup-failed", "query-failed", "discover-failed", "wrong-path", "wrong-model", "missing-settings", "short-settings", "bad-settings"} {
		t.Run(name, func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			args := []string{"key", "f13"}
			if name == "rgb-write" {
				args = []string{"rgb", "steady", "0000ff"}
			}
			if strings.HasPrefix(name, "query-") && name != "query-failed" {
				args = []string{"key"}
			}
			if name == "query-rgb" {
				args = []string{"rgb"}
			}
			if name == "no-op" {
				args = []string{"key", "enter"}
			}
			if name == "noninteractive-selection" {
				args = []string{"key"}
			}
			input := "y\n"
			multiple := name == "pasted-crlf" || name == "pasted-selection" || name == "second-device" || strings.Contains(name, "selection") || name == "query-multiple" || name == "out-of-range"
			if multiple {
				input = "1\ny\n"
			}
			switch name {
			case "pasted-crlf":
				input = "1\r\ny\r\n"
			case "second-device":
				input = "2\ny\n"
			case "confirm-blank":
				input = "\n"
			case "confirm-upper":
				input = "Y\n"
			case "confirm-yes":
				input = "yes\n"
			case "confirm-yes-upper":
				input = "YES\n"
			case "cancel":
				input = "n\n"
			case "cancel-upper":
				input = "N\n"
			case "cancel-no":
				input = "no\n"
			case "cancel-no-upper":
				input = "NO\n"
			case "old-confirm":
				input = "WRITE\n"
			case "space-only":
				input = " \n"
			case "blank-selection":
				input = "\ny\n"
			case "eof", "eof-selection":
				input = ""
			case "partial-write":
				input = "yes"
			case "partial-selection":
				input = "1"
			case "invalid-confirm", "invalid-selection":
				input = "invalid\ny\n"
			case "space-confirm":
				input = " y\n"
			case "out-of-range":
				input = "3\ny\n"
			}
			first := newSettingsTransport()
			if name == "unsupported" {
				first.current[3] = 2
			}
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			if name == "clear-modifiers" {
				first.current[2], fresh.current[2] = 15, 15
			}
			var out, diagnostic bytes.Buffer
			queried, applied, discovered := false, false, false
			wantPath := "1-1.2"
			if name == "second-device" {
				wantPath = "1-1.3"
			}
			access := publicAccess{
				interactive: func(io.Reader) bool {
					return name != "noninteractive" && name != "noninteractive-selection" && name != "query-key" && name != "query-rgb"
				},
				discover: func() ([]Candidate, error) {
					discovered = true
					if name == "discover-failed" {
						return nil, fmt.Errorf("discovery failed")
					}
					if name == "zero" {
						return nil, nil
					}
					cs := []Candidate{{PhysicalPath: "1-1.2", Compatible: name != "incompatible"}}
					if multiple {
						cs = append(cs, Candidate{PhysicalPath: "1-1.3", Compatible: true})
					}
					return cs, nil
				},
				query: func(c Candidate) (CaptureResult, error) {
					queried = true
					if c.PhysicalPath != wantPath {
						t.Fatal(c)
					}
					if name == "query-failed" {
						return CaptureResult{}, fmt.Errorf("query failed")
					}
					q, err := queryCapture(first, true)
					q.result.PhysicalPath = c.PhysicalPath
					switch name {
					case "wrong-path":
						q.result.PhysicalPath = "1-9"
					case "wrong-model":
						q.result.Identity.Model = 0
					case "missing-settings":
						q.result.Readback = nil
					case "short-settings":
						q.result.Readback.Configuration = "00"
					case "bad-settings":
						q.result.Readback.Configuration = "zz"
					}
					return q.result, err
				},
				apply: func(c Candidate, r string, target ApplyTarget, changes Changes, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					applied = true
					if c.PhysicalPath != wantPath || r != root || target != settingsTarget() || !strings.Contains(out.String(), " -> ") || !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
						t.Fatal(c, r, target, out.String())
					}
					switch name {
					case "identity-changed":
						fresh.identity[6] ^= 1
					case "version-changed":
						fresh.identity[4] ^= 1
					case "settings-changed":
						fresh.current[63] ^= 1
					case "became-no-op":
						fresh.current, _ = changeConfiguration(fresh.current, changes)
					case "mismatch":
						fresh.mismatch = true
					case "echo":
						fresh.failAt, fresh.failure = 5, "echo"
					case "deadline":
						fresh.failAt, fresh.failure = 5, "read-deadline"
					}
					dir, err := newCapture(root, c)
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					if name == "backup-failed" {
						if err := saveExclusive(dir, "reply-01.bin", []byte("occupied")); err != nil {
							t.Fatal(err)
						}
					}
					return applySettingsConfirmed(fresh, dir, target, changes, true, time.Second, guard, true)
				},
			}
			err := runPublic(args, &cliRuntime{strings.NewReader(input), &out, &diagnostic}, access)
			failure := name == "zero" || name == "incompatible" || strings.HasPrefix(name, "noninteractive") || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "mismatch" || name == "echo" || name == "deadline" || name == "unsupported" || strings.HasSuffix(name, "failed") || strings.HasPrefix(name, "wrong-") || strings.HasSuffix(name, "settings")
			if (err != nil) != failure {
				t.Fatal(err, out.String(), diagnostic.String())
			}
			success := name == "rgb-write" || name == "clear-modifiers" || name == "pasted-crlf" || strings.HasPrefix(name, "confirm") || name == "pasted-selection" || name == "second-device"
			wantApply := success || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "mismatch" || name == "echo" || name == "deadline" || name == "backup-failed"
			if applied != wantApply {
				t.Fatal("apply", applied, err)
			}
			noQuery := name == "zero" || name == "incompatible" || name == "noninteractive" || strings.HasSuffix(name, "selection") && name != "pasted-selection" || name == "out-of-range" || name == "discover-failed"
			if queried == noQuery {
				t.Fatal("query", queried, err)
			}
			if name == "noninteractive" && discovered {
				t.Fatal("noninteractive write discovered devices")
			}
			checkNoUpload(t, first)
			if !wantApply || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "backup-failed" {
				checkNoUpload(t, fresh.settingsTransport)
			}
			if !wantApply {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal(entries, err)
				}
			}
			if success {
				if !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
					t.Fatal("confirmation prompt missing", out.String())
				}
				if !fresh.checked || len(fresh.packets) != 11 {
					t.Fatal("durable backup/write missing")
				}
				if !strings.HasSuffix(out.String(), "Settings saved. All 128 readback bytes match.\n") || strings.Contains(out.String(), "Records") {
					t.Fatal(out.String())
				}
			}
			if name == "clear-modifiers" && fresh.current[2] != 0 {
				t.Fatal("modifiers were not cleared")
			}
			if name == "rgb-write" {
				for i := range 124 {
					if first.current[i] != fresh.current[i] {
						t.Fatal("RGB changed key/unrelated byte", i)
					}
				}
			}
			if name == "query-key" && (strings.Contains(out.String(), "Colour") || strings.Contains(out.String(), "Mode")) {
				t.Fatal(out.String())
			}
			if name == "query-rgb" && (strings.Contains(out.String(), "\n  Key ") || strings.Contains(out.String(), "\n  On ")) {
				t.Fatal(out.String())
			}
			for _, token := range []string{"wonkey-target-v1:", "Apply result", "Target", "Warning: this changes stored settings.", "Persistence after reconnect"} {
				if strings.Contains(out.String(), token) {
					t.Fatal(out.String())
				}
			}
		})
	}
}

func TestSelectedCandidatePin(t *testing.T) {
	original := Candidate{PhysicalPath: "1-1.2", SysfsPath: "/synthetic", Revision: "0100", Compatible: true, USBDescriptor: usbDescriptorHex, Reports: reportDescriptorHex, VendorNode: NodeMetadata{Path: "/synthetic/hidraw4", identity: [3]uint64{1, 2, 3}}}
	if !sameCandidate(original, original) {
		t.Fatal("self mismatch")
	}
	for _, mutate := range []func(*Candidate){
		func(c *Candidate) { c.PhysicalPath = "1-1.3" }, func(c *Candidate) { c.SysfsPath = "/other" }, func(c *Candidate) { c.Revision = "0200" }, func(c *Candidate) { c.Compatible = false }, func(c *Candidate) { c.USBDescriptor = "bad" }, func(c *Candidate) { c.Reports[3] = "bad" }, func(c *Candidate) { c.VendorNode.Path = "/synthetic/hidraw5" }, func(c *Candidate) { c.VendorNode.identity[1]++ },
	} {
		changed := original
		mutate(&changed)
		if sameCandidate(original, changed) {
			t.Fatal("accepted changed candidate")
		}
	}
}

func TestPublicRGBReadDoesNotCreateBackupRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	t.Setenv("XDG_STATE_HOME", root)
	access := publicAccess{interactive: func(io.Reader) bool { return false }, discover: func() ([]Candidate, error) { return []Candidate{{PhysicalPath: "1-1.2", Compatible: true}}, nil }, query: func(c Candidate) (CaptureResult, error) {
		q, err := queryCapture(newSettingsTransport(), true)
		q.result.PhysicalPath = c.PhysicalPath
		return q.result, err
	}}
	if err := runPublic([]string{"rgb"}, &cliRuntime{strings.NewReader(""), io.Discard, io.Discard}, access); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("query created root", err)
	}
}

func TestPublicLiveBoundaryOffline(t *testing.T) {
	for _, name := range []string{"write", "reordered", "descriptor", "node", "inode", "path", "missing", "open-failed"} {
		t.Run(name, func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			oldDiscover, oldOpen := liveDiscover, liveOpenTarget
			t.Cleanup(func() { liveDiscover, liveOpenTarget = oldDiscover, oldOpen })
			selected := Candidate{PhysicalPath: "1-1.2", Compatible: true, VendorNode: NodeMetadata{Path: "/synthetic/hidraw4", identity: [3]uint64{1, 2, 3}}}
			other := Candidate{PhysicalPath: "1-1.3", Compatible: true}
			discoveries, opens := 0, 0
			liveDiscover = func(string, string) ([]Candidate, error) {
				discoveries++
				if discoveries == 1 {
					return []Candidate{selected, other}, nil
				}
				fresh := selected
				switch name {
				case "descriptor":
					fresh.Reports[3] = "changed"
				case "node":
					fresh.VendorNode.Path = "/synthetic/hidraw5"
				case "inode":
					fresh.VendorNode.identity[1]++
				case "path":
					fresh.PhysicalPath = "1-9"
				case "missing":
					return nil, nil
				}
				if name == "write" {
					return []Candidate{fresh, other}, nil
				}
				return []Candidate{other, fresh}, nil
			}
			first := newSettingsTransport()
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			liveOpenTarget = func(c Candidate) (queryTransport, io.Closer, error) {
				opens++
				if !sameCandidate(c, selected) {
					t.Fatal("opened a different selection", c)
				}
				if opens == 1 {
					return first, &closeRecorder{}, nil
				}
				if name == "open-failed" {
					return nil, nil, fmt.Errorf("open failed")
				}
				entries, err := os.ReadDir(root)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.IsDir() {
						fresh.dir = filepath.Join(root, entry.Name()[:13]+"_"+captureSettingsLabel(fresh.current[:]))
					}
				}
				if fresh.dir == "" {
					t.Fatal("backup directory not created before open")
				}
				return fresh, &closeRecorder{}, nil
			}
			access := publicDeviceAccess()
			access.interactive = func(io.Reader) bool { return true }
			var out bytes.Buffer
			err := runPublic([]string{"key", "f13"}, &cliRuntime{strings.NewReader("1\ny\n"), &out, io.Discard}, access)
			success := name == "write" || name == "reordered"
			if (err == nil) != success {
				t.Fatal(err, out.String())
			}
			if discoveries != 2 {
				t.Fatal("selection was not revalidated", discoveries)
			}
			wantOpens := 1
			if success || name == "open-failed" {
				wantOpens = 2
			}
			if opens != wantOpens {
				t.Fatal("unexpected HID opens", opens)
			}
			checkNoUpload(t, first)
			if success {
				if !fresh.checked || !strings.Contains(out.String(), "All 128 readback bytes match") {
					t.Fatal("backup or verification missing", out.String())
				}
			} else {
				checkNoUpload(t, fresh.settingsTransport)
			}
		})
	}
}
