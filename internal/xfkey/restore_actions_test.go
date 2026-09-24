package xfkey

import (
	"bufio"
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

func TestRestoreActionTransitions(t *testing.T) {
	actions := append(actionExamples(), MultiAction{Interval: 1, Repeat: 1, Keys: bytes.Repeat([]byte{4}, 115)})
	for i, old := range actions {
		for j, next := range actions {
			t.Run(fmt.Sprintf("%d-to-%d", i, j), func(t *testing.T) {
				current := actionConfiguration(t, old)
				saved := actionConfiguration(t, next)
				current[120], saved[120] = 0xaa, 0xbb
				saved[124], saved[125], saved[126], saved[127] = 6, 1, 2, 3
				changes, err := restoreChanges(saved)
				if err != nil {
					t.Fatal(err)
				}
				intended, err := changes.configuration(current)
				if err != nil {
					t.Fatal(err)
				}
				oldBytes, _ := old.actionBytes()
				newBytes, _ := next.actionBytes()
				want := current
				clear(want[:len(oldBytes)])
				copy(want[:], newBytes)
				copy(want[124:], saved[124:])
				if intended != want {
					t.Fatal("restore copied inactive source bytes or missed active fields")
				}
				decoded, err := decodeAction(intended)
				if err != nil || !reflect.DeepEqual(decoded, next) {
					t.Fatal(decoded, err)
				}
				root := publicTestCaptureRoot(t)
				sourceTransport, first := newSettingsTransport(), newSettingsTransport()
				sourceTransport.current, first.current = saved, current
				source := restoreTestSource(t, t.TempDir(), "source", sourceTransport)
				fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
				fresh.current = current
				access := publicActionTestAccess(t, first, func(candidate Candidate, destination string, target ApplyTarget, request configurationChanges, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					if destination != root {
						t.Fatal("wrong backup destination")
					}
					dir, err := newCapture(destination, candidate)
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					return applySettingsConfirmed(fresh, dir, target, request, true, time.Second, guard, true)
				})
				var out bytes.Buffer
				err = runPublic([]string{"restore", source}, &cliRuntime{strings.NewReader("\n"), &out, io.Discard}, access)
				if err != nil || !fresh.checked || fresh.current != want || len(fresh.packets) != 11 {
					t.Fatal("restore missed guarded transaction", err, out.String())
				}
				if !strings.Contains(out.String(), "Other saved bytes differ") || !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
					t.Fatal("missing preservation warning or confirmation", out.String())
				}
			})
		}
	}
}

func TestRestoreTypedSources(t *testing.T) {
	for _, action := range actionExamples() {
		for _, name := range []string{"moved", "failed", "corrupt-config", "corrupt-reply", "corrupt-result", "missing-result", "linked-directory", "linked-file", "oversized-file"} {
			t.Run(fmt.Sprintf("%T/%s", action, name), func(t *testing.T) {
				f := newSettingsTransport()
				f.current = actionConfiguration(t, action)
				dir := restoreTestSource(t, t.TempDir(), "source", f)
				switch name {
				case "moved":
					moved := filepath.Join(t.TempDir(), "moved")
					if err := os.Rename(dir, moved); err != nil {
						t.Fatal(err)
					}
					dir = moved
				case "failed":
					failed := newSettingsTransport()
					failed.current = f.current
					failed.failAt, failed.failure = 8, "echo"
					dir = settingsDir(t)
					result, err := applySettings(failed, dir, settingsTarget(), ActionChanges{RGB: Changes{"red": 1}}, true, time.Second)
					if err == nil || !result.WriteAttempted {
						t.Fatal("transaction did not fail after upload", result, err)
					}
				case "corrupt-config", "corrupt-reply", "corrupt-result", "oversized-file":
					file := map[string]string{"corrupt-config": "configuration.bin", "corrupt-reply": "reply-07.bin", "corrupt-result": "result.json", "oversized-file": "reply-08.bin"}[name]
					data := []byte("corrupt")
					if name == "oversized-file" {
						data = make([]byte, 65)
					}
					if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
						t.Fatal(err)
					}
				case "missing-result":
					if err := os.Remove(filepath.Join(dir, "result.json")); err != nil {
						t.Fatal(err)
					}
				case "linked-directory":
					link := filepath.Join(t.TempDir(), "link")
					if err := os.Symlink(dir, link); err != nil {
						t.Fatal(err)
					}
					dir = link
				case "linked-file":
					file := filepath.Join(dir, "reply-08.bin")
					if err := os.Rename(file, file+".original"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(file+".original", file); err != nil {
						t.Fatal(err)
					}
				}
				source, err := loadRestoreSource(dir)
				valid := name == "moved" || name == "failed"
				if (err == nil) != valid || (valid && source.config != f.current) {
					t.Fatal("incorrect source validation", source, err)
				}
			})
		}
	}
}

func TestRestoreTypedSourceList(t *testing.T) {
	root := publicTestCaptureRoot(t)
	current := newSettingsTransport()
	identity, err := parseIdentity(current.identity)
	if err != nil {
		t.Fatal(err)
	}
	for i, action := range actionExamples() {
		saved := newSettingsTransport()
		saved.current = actionConfiguration(t, action)
		saved.current[120] ^= 1
		restoreTestSource(t, root, fmt.Sprintf("2026091%d-123456-0000", i), saved)
	}
	for _, action := range actionExamples() {
		current.current = actionConfiguration(t, action)
		sources, err := listRestoreSources(root, identity, current.current)
		if err != nil || len(sources) != 3 {
			t.Fatal("did not exclude matching action and RGB", sources, err)
		}
		for i, source := range sources {
			if source.config[0] == current.current[0] || (i > 0 && source.created.After(sources[i-1].created)) {
				t.Fatal("incorrect selection or order")
			}
		}
		var out bytes.Buffer
		selected, err := chooseRestore("", identity, current.current, bufio.NewReader(strings.NewReader("1\n")), newHuman(&out))
		if err != nil || selected == nil || selected.directory != sources[0].directory {
			t.Fatal(selected, err)
		}
		for i, source := range sources {
			action, _ := decodeAction(source.config)
			label := strings.TrimPrefix(actionDescription(action), "keyboard ")
			row := fmt.Sprintf("  %d  %s | ", i+1, label)
			if !strings.Contains(out.String(), row) || strings.Contains(out.String(), filepath.Base(source.directory)) {
				t.Fatal("missing action label or exposed directory", out.String())
			}
		}
	}
}

func TestRestorePreservedBytesWarning(t *testing.T) {
	current := actionConfiguration(t, MouseAction{Buttons: 1, X: 2, Y: 3, Wheel: 4})
	source := restoreSource{config: actionConfiguration(t, MediaAction{Usage: 0xe9})}
	changes, err := restoreChanges(source.config)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := changes.configuration(current)
	if err != nil {
		t.Fatal(err)
	}
	if source.hasDifferentPreservedBytes(current, restored) {
		t.Fatal("warned that cleared former action bytes remain unchanged")
	}
	source.config[120] ^= 1
	if !source.hasDifferentPreservedBytes(current, restored) {
		t.Fatal("missing preserved unknown bytes warning")
	}
}

func TestRestoreInvalidRGB(t *testing.T) {
	for _, action := range actionExamples() {
		for _, mode := range []byte{0, 9, 255} {
			t.Run(fmt.Sprintf("%T/%d", action, mode), func(t *testing.T) {
				valid := actionConfiguration(t, action)
				invalid := valid
				invalid[124] = mode
				if _, err := restoreChanges(invalid); err == nil {
					t.Fatal("accepted invalid source RGB")
				}
				changes, err := restoreChanges(valid)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := changes.configuration(invalid); err == nil {
					t.Fatal("accepted invalid current RGB")
				}
			})
		}
	}
}

func TestRestoreInvalidAndAmbiguousLayouts(t *testing.T) {
	for _, raw := range [][]byte{{255}, {0, 1, 0, 2, 4}, {1, 8, 0, 0, 0}, {1, 0, 128, 0, 0}, {2, 0, 0}, {2, 0x3d, 2}, {3, 0, 0, 1, 1, 4}, {3, 0, 1, 0, 1, 4}, {3, 0, 1, 1, 116}} {
		for _, invalidSource := range []bool{false, true} {
			t.Run(fmt.Sprintf("%x/source=%v", raw, invalidSource), func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				saved, current := newSettingsTransport(), newSettingsTransport()
				if invalidSource {
					copy(saved.current[:], raw)
				} else {
					copy(current.current[:], raw)
				}
				source := restoreTestSource(t, t.TempDir(), "source", saved)
				input := strings.NewReader("yes\n")
				err := runPublic([]string{"restore", source}, &cliRuntime{input, io.Discard, io.Discard}, publicActionTestAccess(t, current, nil))
				if err == nil || input.Len() != len("yes\n") {
					t.Fatal("invalid layout reached confirmation", err)
				}
				checkNoUpload(t, current)
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal("invalid layout created backup", entries, err)
				}
			})
		}
	}
	for _, action := range actionExamples()[1:] {
		for _, keyboardSource := range []bool{false, true} {
			t.Run(fmt.Sprintf("%T/keyboard-source=%v", action, keyboardSource), func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				current, saved := newSettingsTransport(), newSettingsTransport()
				current.current, saved.current = restoreTestSettings(), actionConfiguration(t, action)
				if keyboardSource {
					current.current, saved.current = saved.current, current.current
				}
				source := restoreTestSource(t, root, "source", saved)
				identity, err := parseIdentity(current.identity)
				if err != nil {
					t.Fatal(err)
				}
				sources, err := listRestoreSources(root, identity, current.current)
				if err != nil || len(sources) != 0 {
					t.Fatal("ambiguous transition listed", sources, err)
				}
				input := strings.NewReader("yes\n")
				err = runPublic([]string{"restore", source}, &cliRuntime{input, io.Discard, io.Discard}, publicActionTestAccess(t, current, nil))
				if err == nil || input.Len() != len("yes\n") {
					t.Fatal("ambiguous transition reached confirmation", err)
				}
				checkNoUpload(t, current)
			})
		}
	}
}
