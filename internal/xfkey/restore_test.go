package xfkey

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func restoreTestSource(t *testing.T, root, name string, transport *settingsTransport) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := captureQueries(transport, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	return result.Directory
}

func restoreTestSettings() configuration {
	c := syntheticSettings()
	c[1], c[2], c[4] = 3, 0, 0x68
	c[124], c[125], c[126], c[127] = 8, 12, 34, 56
	return c
}

func TestRestoreOutputFailure(t *testing.T) {
	for _, short := range []bool{false, true} {
		for _, tc := range []struct {
			name, needle string
			explicit     bool
		}{
			{"selection-heading", "Restore a backup:", false},
			{"selection-row", "f13 (both)", false},
			{"selection-prompt", "Backup number", false},
			{"source", "Source", true},
			{"preview", "Restore saved key", true},
			{"changes", " -> ", false},
			{"confirmation", "Save settings?", false},
		} {
			t.Run(fmt.Sprintf("%s/short=%v", tc.name, short), func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				saved := newSettingsTransport()
				saved.current = restoreTestSettings()
				source := restoreTestSource(t, root, "source", saved)
				args, input := []string{"restore"}, "1\n\n"
				if tc.explicit {
					args, input = append(args, source), "\n"
				}
				checkPublicOutputFailure(t, root, args, input, tc.needle, false, short)
			})
		}
	}
}

func TestRestoreSourceValidation(t *testing.T) {
	for _, name := range []string{"complete", "moved", "legacy-gui-label", "failed-transaction", "missing-completion", "corrupt-configuration", "corrupt-reply", "linked-directory", "linked-reply", "unsupported"} {
		t.Run(name, func(t *testing.T) {
			f := newSettingsTransport()
			if name == "legacy-gui-label" {
				f.current[2] = 8
			}
			if name == "unsupported" {
				f.current[3] = 2
			}
			dir := restoreTestSource(t, t.TempDir(), "source", f)
			original := dir
			switch name {
			case "moved", "legacy-gui-label":
				dir = filepath.Join(t.TempDir(), "moved")
				if name == "legacy-gui-label" {
					dir = filepath.Join(t.TempDir(), "260912-083853_key-gui-enter_rgb-gradient-ffffff")
				}
				if err := os.Rename(original, dir); err != nil {
					t.Fatal(err)
				}
			case "failed-transaction":
				dir = settingsDir(t)
				failed := newSettingsTransport()
				failed.failAt, failed.failure = 8, "echo"
				result, err := applySettings(failed, dir, settingsTarget(), Changes{"key": 0x68}, true, time.Second)
				if err == nil || !result.WriteAttempted || result.Outcome != "failed-state-unknown" {
					t.Fatal("fixture did not fail after upload", result, err)
				}
			case "missing-completion":
				if err := os.Remove(filepath.Join(dir, "result.json")); err != nil {
					t.Fatal(err)
				}
			case "corrupt-configuration", "corrupt-reply":
				file := "configuration.bin"
				if name == "corrupt-reply" {
					file = "reply-07.bin"
				}
				path := filepath.Join(dir, file)
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				b[20] ^= 1
				// #nosec G703 -- The path uses a temporary test directory and one of two fixed filenames.
				if err := os.WriteFile(path, b, 0o600); err != nil {
					t.Fatal(err)
				}
			case "linked-directory":
				dir = filepath.Join(t.TempDir(), "linked")
				if err := os.Symlink(original, dir); err != nil {
					t.Fatal(err)
				}
			case "linked-reply":
				path := filepath.Join(dir, "reply-07.bin")
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".original", path); err != nil {
					t.Fatal(err)
				}
			}
			source, err := loadRestoreSource(dir)
			valid := name == "complete" || name == "moved" || name == "legacy-gui-label" || name == "failed-transaction"
			if (err == nil) != valid {
				t.Fatalf("source=%#v error=%v", source, err)
			}
			if valid && source.config != f.current {
				t.Fatal("source bytes changed")
			}
		})
	}
}

func TestPublicRestoreWorkflow(t *testing.T) {
	for _, name := range []string{"write", "blank", "yes", "y", "upper", "old-confirm", "no", "invalid", "leading-space", "trailing-space", "eof", "partial", "nonterminal", "no-op", "settings-drift", "became-no-op", "identity-drift", "version-drift", "backup-failure", "backup-corrupt", "upload-failure", "commit-failure", "deadline", "mismatch", "incompatible-identifier", "incompatible-version", "unsupported-current", "source-replaced"} {
		t.Run(name, func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			saved := newSettingsTransport()
			saved.current = restoreTestSettings()
			if name == "incompatible-identifier" {
				saved.identity[6] ^= 1
			}
			if name == "incompatible-version" {
				saved.identity[4] ^= 1
			}
			source := restoreTestSource(t, t.TempDir(), "outside-storage", saved)
			first := newSettingsTransport()
			first.current[2], first.current[63] = 15, 231
			if name == "no-op" {
				first.current = saved.current
				first.current[63] ^= 1
			}
			if name == "unsupported-current" {
				first.current[3] = 2
			}
			original := first.current
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			fresh.current = original
			input := "y\n"
			switch name {
			case "blank":
				input = "\n"
			case "yes":
				input = "yes\n"
			case "y":
				input = "y\n"
			case "upper":
				input = "YES\n"
			case "old-confirm":
				input = "WRITE\n"
			case "no":
				input = "no\n"
			case "invalid":
				input = "invalid\n"
			case "leading-space":
				input = " y\n"
			case "trailing-space":
				input = "yes \n"
			case "eof":
				input = ""
			case "partial":
				input = "yes"
			}
			applied, discovered := false, false
			var out bytes.Buffer
			access := publicAccess{
				interactive: func(io.Reader) bool { return name != "nonterminal" },
				discover: func() ([]Candidate, error) {
					discovered = true
					return []Candidate{{PhysicalPath: "1-2", Compatible: true}}, nil
				},
				query: func(c Candidate) (CaptureResult, error) {
					q, err := queryCapture(first, true)
					q.result.PhysicalPath = c.PhysicalPath
					return q.result, err
				},
				apply: func(c Candidate, destination string, target ApplyTarget, changes Changes, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					applied = true
					if destination != root || destination == source || target != settingsTarget() {
						t.Fatal("wrong backup destination or target")
					}
					if !strings.Contains(out.String(), source) || !strings.Contains(out.String(), " -> ") {
						t.Fatal("source or preview missing", out.String())
					}
					switch name {
					case "settings-drift":
						fresh.current[63] ^= 1
					case "became-no-op":
						fresh.current, _ = changeConfiguration(fresh.current, changes)
					case "identity-drift":
						fresh.identity[6] ^= 1
					case "version-drift":
						fresh.identity[4] ^= 1
					case "upload-failure":
						fresh.failAt, fresh.failure = 5, "echo"
					case "commit-failure":
						fresh.failAt, fresh.failure = 8, "echo"
					case "deadline":
						fresh.failAt, fresh.failure = 5, "read-deadline"
					case "mismatch":
						fresh.mismatch = true
					case "source-replaced":
						if err := os.WriteFile(filepath.Join(source, "configuration.bin"), make([]byte, 128), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					dir, err := newCapture(root, c)
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					if name == "backup-failure" {
						if err := saveExclusive(dir, "reply-01.bin", []byte("occupied")); err != nil {
							t.Fatal(err)
						}
					}
					if name == "backup-corrupt" {
						return applySettingsWithCapture(fresh, dir, target, changes, true, time.Second, guard, func(tr queryTransport, path string, readback bool) (CaptureResult, error) {
							r, err := captureQueries(tr, path, readback)
							if err == nil {
								err = os.WriteFile(filepath.Join(r.Directory, "configuration.bin"), make([]byte, 128), 0o600)
							}
							return r, err
						}, true)
					}
					return applySettingsConfirmed(fresh, dir, target, changes, true, time.Second, guard, true)
				},
			}
			err := runPublic([]string{"restore", source}, &cliRuntime{strings.NewReader(input), &out, io.Discard}, access)
			failed := strings.Contains(name, "drift") || strings.Contains(name, "failure") || strings.Contains(name, "incompatible") || name == "became-no-op" || name == "deadline" || name == "mismatch" || name == "nonterminal" || name == "unsupported-current" || name == "backup-corrupt"
			if (err != nil) != failed {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
			success := name == "write" || name == "source-replaced" || name == "blank" || name == "yes" || name == "y" || name == "upper"
			if name == "nonterminal" && discovered {
				t.Fatal("nonterminal restore accessed devices")
			}
			checkNoUpload(t, first)
			if success {
				if !fresh.checked || len(fresh.packets) != 11 {
					t.Fatal("restore omitted fresh backup or transaction")
				}
				want := original
				for _, offset := range []int{1, 2, 4, 124, 125, 126, 127} {
					want[offset] = saved.current[offset]
				}
				if fresh.current != want {
					t.Fatal("restore changed unknown bytes or missed saved fields")
				}
				_, backup, err := loadCapture(fresh.dir)
				if err != nil || backup != original {
					t.Fatal("new backup does not preserve pre-restore state", err)
				}
			} else if name != "upload-failure" && name != "commit-failure" && name != "deadline" && name != "mismatch" {
				checkNoUpload(t, fresh.settingsTransport)
			}
			if !applied {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal("cancelled or rejected restore created a backup", entries, err)
				}
			}
		})
	}
}

func TestRestoreSourceList(t *testing.T) {
	root := publicTestCaptureRoot(t)
	current := newSettingsTransport()
	identity, err := parseIdentity(current.identity)
	if err != nil {
		t.Fatal(err)
	}
	var eligible []string
	for _, name := range []string{"20260910-123456-0000", "20260912-123456-0000", "20260911-123456-0000", "equivalent", "incompatible", "incomplete", "linked"} {
		f := newSettingsTransport()
		f.current = restoreTestSettings()
		if name == "equivalent" {
			f.current = current.current
			f.current[63] ^= 1
		}
		if name == "incompatible" {
			f.identity[6] ^= 1
		}
		dir := restoreTestSource(t, root, name, f)
		if name == "incomplete" {
			if err := os.Remove(filepath.Join(dir, "result.json")); err != nil {
				t.Fatal(err)
			}
		}
		if name == "linked" {
			moved := filepath.Join(t.TempDir(), name)
			if err := os.Rename(dir, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, dir); err != nil {
				t.Fatal(err)
			}
		}
		if strings.HasPrefix(name, "2026") {
			eligible = append(eligible, dir)
			if err := saveExclusive(dir, "apply-outcome.json", []byte(`{"outcome":"failed-state-unknown"}`)); err != nil {
				t.Fatal(err)
			}
		}
	}
	want := []string{eligible[1], eligible[2], eligible[0]}
	for _, symlinkRoot := range []bool{false, true} {
		path := root
		if symlinkRoot {
			path = filepath.Join(t.TempDir(), "root-link")
			if err := os.Symlink(root, path); err != nil {
				t.Fatal(err)
			}
		}
		sources, err := listRestoreSources(path, identity, current.current)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, source := range sources {
			got = append(got, source.directory)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("sources=%v, want %v", got, want)
		}
	}
	for _, answer := range []string{"\n", "", "1", "0\n", "4\n", "invalid\n", " 1\n"} {
		source, err := chooseRestore("", identity, current.current, bufio.NewReader(strings.NewReader(answer)), newHuman(io.Discard))
		if err != nil || source != nil {
			t.Fatalf("answer=%q source=%#v error=%v", answer, source, err)
		}
	}
}

func TestRestoreSelectionOutput(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("test", 3600)
	t.Cleanup(func() { time.Local = previousLocal })
	for _, count := range []int{1, 2} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			current := newSettingsTransport()
			identity, err := parseIdentity(current.identity)
			if err != nil {
				t.Fatal(err)
			}
			saved := newSettingsTransport()
			saved.current = restoreTestSettings()
			first := restoreTestSource(t, root, "20260912-123456-0000", saved)
			want := "Restore a backup:\n  1  f13 (both) | toggle #0C2238 | 12 Sep 2026 13:34\n"
			if count == 2 {
				saved.current[1], saved.current[2], saved.current[4] = 2, 11, 0x28
				saved.current[124], saved.current[125], saved.current[126], saved.current[127] = 2, 255, 0, 0
				restoreTestSource(t, root, "20260911-093012-0000", saved)
				want += "  2  ctrl+shift+super+enter (release) | steady #FF0000 | 11 Sep 2026 10:30\n"
			}
			want += "Backup number [cancel]: "
			var out bytes.Buffer
			source, err := chooseRestore("", identity, current.current, bufio.NewReader(strings.NewReader("1\n")), newHuman(&out))
			if err != nil || source == nil || source.directory != first {
				t.Fatalf("selection=%#v error=%v", source, err)
			}
			if out.String() != want {
				t.Fatalf("output=%q, want %q", out.String(), want)
			}
		})
	}
}

func TestRestoreWithoutEligibleCapturesDoesNotApply(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	access := publicAccess{
		interactive: func(io.Reader) bool { return true },
		discover:    func() ([]Candidate, error) { return []Candidate{{PhysicalPath: "1-2", Compatible: true}}, nil },
		query: func(c Candidate) (CaptureResult, error) {
			q, err := queryCapture(newSettingsTransport(), true)
			q.result.PhysicalPath = c.PhysicalPath
			return q.result, err
		},
		apply: func(Candidate, string, ApplyTarget, Changes, func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
			t.Fatal("restore applied without an eligible capture")
			return ApplyResult{}, nil
		},
	}
	var out bytes.Buffer
	if err := runPublic([]string{"restore"}, &cliRuntime{strings.NewReader("write\n"), &out, io.Discard}, access); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No compatible captures") {
		t.Fatal(out.String())
	}
	entries, err := os.ReadDir(state)
	if err != nil || len(entries) != 0 {
		t.Fatal("restore created storage without a source", entries, err)
	}
}

func TestRestoreSharesSelectionAndConfirmationReader(t *testing.T) {
	for _, input := range []string{"2\n1\ny\n", "2\r\n1\r\ny\r\n", "2\ny\n", "2\n\n", "2\n1\n\n", "2\n1\nn\n", "2\n1\nyes"} {
		t.Run(strings.ReplaceAll(input, "\n", "/"), func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			saved := newSettingsTransport()
			saved.current = restoreTestSettings()
			source := restoreTestSource(t, root, "20260912-123456-0000", saved)
			before, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			applied := false
			var out bytes.Buffer
			access := publicAccess{
				interactive: func(io.Reader) bool { return true },
				discover: func() ([]Candidate, error) {
					return []Candidate{{PhysicalPath: "1-2", Compatible: true}, {PhysicalPath: "1-3", Compatible: true}}, nil
				},
				query: func(c Candidate) (CaptureResult, error) {
					if c.PhysicalPath != "1-3" {
						t.Fatal("selected wrong device")
					}
					q, err := queryCapture(newSettingsTransport(), true)
					q.result.PhysicalPath = c.PhysicalPath
					return q.result, err
				},
				apply: func(c Candidate, destination string, target ApplyTarget, changes Changes, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					applied = true
					if !strings.Contains(out.String(), "Backup number") || strings.Contains(out.String(), source) {
						t.Fatal("backup selection missing or source path repeated", out.String())
					}
					dir, err := newCapture(destination, c)
					if err != nil {
						return ApplyResult{}, err
					}
					return applySettingsConfirmed(newSettingsTransport(), dir, target, changes, true, time.Second, guard, true)
				},
			}
			if err := runPublic([]string{"restore"}, &cliRuntime{strings.NewReader(input), &out, io.Discard}, access); err != nil {
				t.Fatal(err)
			}
			wantApply := input == "2\n1\ny\n" || input == "2\r\n1\r\ny\r\n" || input == "2\n1\n\n"
			if applied != wantApply {
				t.Fatalf("applied=%t input=%q output=%s", applied, input, out.String())
			}
			if !applied {
				after, err := os.ReadDir(root)
				if err != nil || len(after) != len(before) {
					t.Fatal("cancellation created a backup", err)
				}
			}
		})
	}
}
