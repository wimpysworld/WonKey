package xfkey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func parseSet(t *testing.T, args ...string) *setCommand {
	t.Helper()
	model := &cliModel{}
	parser, err := kong.New(model)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := parser.Parse(append([]string{"set"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	if err := adaptCLI(model, ctx); err != nil {
		t.Fatal(err)
	}
	return &model.Set
}

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

func TestSetWorkflow(t *testing.T) {
	for _, name := range []string{"yes", "confirm", "cancel", "noninteractive", "dry-run", "no-op", "identity-changed", "version-changed", "settings-changed", "became-no-op", "mismatch", "echo", "deadline", "unsupported", "backup-failed"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"key=f13", "light=steady:0000ff", "--capture-root", root, "--json"}
			if name == "dry-run" {
				args = append(args, "--dry-run")
			} else if name != "confirm" && name != "cancel" && name != "noninteractive" {
				args = append(args, "--yes")
			}
			if name == "no-op" {
				args[0], args[1] = "key=enter", "light=gradient:ffffff"
			}
			command := parseSet(t, args...)
			var out, diagnostics bytes.Buffer
			input := "write\n"
			if name == "cancel" {
				input = "\n"
			}
			rt := &cliRuntime{in: strings.NewReader(input), out: &out, errOut: &diagnostics}
			queried, applied := false, false
			first := newSettingsTransport()
			if name == "unsupported" {
				first.current[3] = 2
			}
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			access := setAccess{
				interactive: func(io.Reader) bool { return name != "noninteractive" && name != "dry-run" },
				query: func(path string, readback bool) (CaptureResult, error) {
					queried = true
					if path != "" || !readback {
						t.Fatal(path, readback)
					}
					q, err := queryCapture(first, true)
					q.result.PhysicalPath = "1-1.2"
					return q.result, err
				},
				apply: func(path, requestedRoot string, target ApplyTarget, changes Changes, write bool, guard func(configuration, configuration, string) (bool, error), checkNoOp ...bool) (ApplyResult, error) {
					applied = true
					if path != "1-1.2" || requestedRoot != root || target != settingsTarget() || !write || !strings.Contains(diagnostics.String(), "Changes:") {
						t.Fatal(path, requestedRoot, target, diagnostics.String())
					}
					if name == "confirm" && !strings.Contains(diagnostics.String(), "Type \"write\"") {
						t.Fatal("confirmation missing")
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
					dir, err := newCapture(root, Candidate{PhysicalPath: path})
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					if name == "backup-failed" {
						if err := saveExclusive(dir, "reply-01.bin", []byte("occupied")); err != nil {
							t.Fatal(err)
						}
					}
					return applySettingsConfirmed(fresh, dir, target, changes, write, time.Second, guard, checkNoOp...)
				},
			}
			err := command.run(rt, access)
			failure := name == "noninteractive" || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "mismatch" || name == "echo" || name == "deadline" || name == "unsupported" || name == "backup-failed"
			if (err != nil) != failure {
				t.Fatalf("error=%v output=%s diagnostics=%s", err, out.String(), diagnostics.String())
			}
			noApply := name == "noninteractive" || name == "dry-run" || name == "cancel" || name == "no-op" || name == "unsupported"
			if applied == noApply || queried == (name == "noninteractive") {
				t.Fatal("unexpected access", queried, applied)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if noApply && len(entries) != 0 {
				t.Fatal("unexpected capture", entries)
			}
			if noApply || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "backup-failed" {
				checkNoUpload(t, fresh.settingsTransport)
			}
			checkNoUpload(t, first)
			if name == "yes" || name == "confirm" {
				if !fresh.checked || len(fresh.packets) != 11 || fresh.current[4] != 0x68 {
					t.Fatal("write or backup check missing")
				}
				for i := 5; i < 124; i++ {
					if fresh.current[i] != first.current[i] {
						t.Fatal("unspecified byte changed", i)
					}
				}
			}
			if out.Len() > 0 {
				var value envelope
				if err := json.Unmarshal(out.Bytes(), &value); err != nil || value.Command != "set" || strings.Contains(out.String(), "\x1b") {
					t.Fatal(out.String(), err)
				}
				if name == "dry-run" && (value.Outcome != "dry-run" || !strings.Contains(diagnostics.String(), "read the device")) {
					t.Fatal(out.String(), diagnostics.String())
				}
			}
		})
	}
}

func TestSetSelection(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		candidates []Candidate
		want       string
		fails      bool
	}{
		{"one", "", []Candidate{{PhysicalPath: "1-1.2", Compatible: true}}, "1-1.2", false},
		{"multiple", "", []Candidate{{PhysicalPath: "1-1.2", Compatible: true}, {PhysicalPath: "1-1.3", Compatible: true}}, "", true},
		{"none", "", nil, "", true},
		{"explicit", "1-1.3", []Candidate{{PhysicalPath: "1-1.2", Compatible: true}, {PhysicalPath: "1-1.3", Compatible: true}}, "1-1.3", false},
		{"rejected", "1-1.2", []Candidate{{PhysicalPath: "1-1.2"}}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := parseSet(t, "key=f13", "--dry-run", "--device", tc.path)
			queried := false
			access := setAccess{query: func(path string, readback bool) (CaptureResult, error) {
				selected, err := selectCandidate(tc.candidates, path)
				if err != nil {
					return CaptureResult{}, err
				}
				queried = true
				if selected.PhysicalPath != tc.want {
					t.Fatal(selected)
				}
				q, err := queryCapture(newSettingsTransport(), readback)
				q.result.PhysicalPath = selected.PhysicalPath
				return q.result, err
			}, apply: func(string, string, ApplyTarget, Changes, bool, func(configuration, configuration, string) (bool, error), ...bool) (ApplyResult, error) {
				t.Fatal("dry-run applied")
				return ApplyResult{}, nil
			}}
			err := command.run(&cliRuntime{out: io.Discard, errOut: io.Discard}, access)
			if (err != nil) != tc.fails || queried == tc.fails {
				t.Fatal(err, queried)
			}
			if tc.name == "multiple" && !strings.Contains(err.Error(), "--device") {
				t.Fatal(err)
			}
		})
	}
}

func TestEverydayQueryOutputs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "must-not-exist")
	t.Setenv("WONKEY_CAPTURE_ROOT", root)
	oldDiscover, oldOpen := queryDiscover, queryOpenTarget
	t.Cleanup(func() { queryDiscover, queryOpenTarget = oldDiscover, oldOpen })
	queryDiscover = func(string, string) ([]Candidate, error) {
		return []Candidate{{PhysicalPath: "1-1.2", Compatible: true}}, nil
	}
	var transport *settingsTransport
	queryOpenTarget = func(candidate Candidate) (queryTransport, io.Closer, error) {
		if candidate.PhysicalPath != "1-1.2" {
			t.Fatal(candidate)
		}
		transport = newSettingsTransport()
		return transport, &closeRecorder{}, nil
	}
	for _, args := range [][]string{{"show"}, {"show", "--target"}, {"show", "--json"}, {"set", "key=f13", "--dry-run"}, {"set", "key=f13", "--dry-run", "--json"}} {
		stdout, stderr, err := runCLI(args...)
		if err != nil {
			t.Fatal(args, stdout, stderr, err)
		}
		checkNoUpload(t, transport)
		if len(transport.packets) != 4 {
			t.Fatal(transport.packets)
		}
		if args[len(args)-1] == "--json" {
			var value envelope
			if err := json.Unmarshal([]byte(stdout), &value); err != nil {
				t.Fatal(stdout, err)
			}
			if args[0] == "show" {
				target, err := decodeTarget(value.Target)
				if err != nil || target.Identifier != "be077ba2" {
					t.Fatal(value, err)
				}
			} else if value.Outcome != "dry-run" || !strings.Contains(strings.Join(value.Warnings, " "), "read the device") {
				t.Fatal(value)
			}
		} else if args[0] == "show" {
			if strings.Contains(stdout, "0112:1-1.2:be077ba2:1014") != (len(args) == 2) {
				t.Fatal(stdout)
			}
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("query created capture root", err)
	}
}

func TestAdvancedAliases(t *testing.T) {
	dir := settingsDir(t)
	if _, err := captureQueries(newSettingsTransport(), dir, true); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plan", dir, "key=f13", "--json"}, {"protocol", "parse-identify", "--hex", fmt.Sprintf("%x", newSettingsTransport().identity)}} {
		legacy, _, err := runCLI(args...)
		if err != nil {
			t.Fatal(err)
		}
		advanced, stderr, err := runCLI(append([]string{"advanced"}, args...)...)
		if err != nil || advanced != legacy || stderr != "" {
			t.Fatal(advanced, stderr, err)
		}
	}
	for _, args := range [][]string{{"advanced", "--help"}, {"advanced", "devices", "--help"}, {"advanced", "plan", "--help"}, {"advanced", "protocol", "--help"}} {
		stdout, stderr, err := runCLI(args...)
		if err != nil || stdout == "" || stderr != "" {
			t.Fatal(args, stdout, stderr, err)
		}
	}
	for _, args := range [][]string{{"set", "--json"}, {"set", "key=f13", "--json"}, {"set", "key=f13", "--write", "--json"}, {"set", "key=f13", "key=enter", "--json"}, {"advanced", "protocol", "parse-identify", "--hex", "bad"}, {"advanced", "plan", "--json"}} {
		stdout, _, err := runCLI(args...)
		if err == nil || !json.Valid([]byte(stdout)) {
			t.Fatal(args, stdout, err)
		}
	}
}
