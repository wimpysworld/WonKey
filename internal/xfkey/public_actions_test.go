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

func TestPublicActionParser(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want Action
	}{
		{[]string{"mouse"}, nil},
		{[]string{"media"}, nil},
		{[]string{"multi"}, nil},
		{[]string{"mouse", "none"}, MouseAction{}},
		{[]string{"mouse", "LEFT+right+middle", "--x", "-127", "--y=127", "--wheel", "-1"}, MouseAction{Buttons: 7, X: -127, Y: 127, Wheel: -1}},
		{[]string{"mouse", "wheelup"}, MouseAction{Wheel: 1}},
		{[]string{"mouse", "wheeldown", "--x=1"}, MouseAction{X: 1, Wheel: -1}},
		{[]string{"media", "0x0001"}, MediaAction{Usage: 1}},
		{[]string{"media", "0x023C"}, MediaAction{Usage: 0x23c}},
		{[]string{"multi", "A,escape,arrowup,lang2"}, MultiAction{Interval: 50, Repeat: 1, Keys: []byte{4, 0x29, 0x52, 0x91}}},
		{[]string{"multi", "f13", "--interval", "1", "--repeat=255"}, MultiAction{Interval: 1, Repeat: 255, Keys: []byte{0x68}}},
		{[]string{"multi", strings.TrimSuffix(strings.Repeat("lang2,", 115), ","), "--interval=65535", "--repeat", "1"}, MultiAction{Interval: 65535, Repeat: 1, Keys: bytes.Repeat([]byte{0x91}, 115)}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := parsePublic(tc.args)
			if err != nil || !reflect.DeepEqual(got.action, tc.want) {
				t.Fatalf("action=%#v want=%#v error=%v", got.action, tc.want, err)
			}
		})
	}
	for key := range keyValues {
		c, err := parsePublic([]string{"multi", key})
		if err != nil || c.action.(MultiAction).Keys[0] != byte(keyValues[key]&255) {
			t.Fatal(key, c, err)
		}
	}
}

func TestPublicMediaNames(t *testing.T) {
	names := []string{"play", "pause", "record", "fastforward", "rewind", "next", "prev", "previous", "stop", "eject", "playpause", "mute", "volumeup", "volumedown", "calculator", "mycomputer", "browser", "email", "search", "home", "back", "forward", "refresh", "bookmarks"}
	codes := []int{0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb6, 0xb7, 0xb8, 0xcd, 0xe2, 0xe9, 0xea, 0x192, 0x194, 0x196, 0x18a, 0x221, 0x223, 0x224, 0x225, 0x227, 0x22a}
	if len(mediaValues) != len(names) {
		t.Fatal("unexpected media table size", len(mediaValues))
	}
	for i, name := range names {
		c, err := parsePublic([]string{"media", strings.ToUpper(name)})
		if err != nil || c.action != (MediaAction{Usage: codes[i]}) {
			t.Fatal(name, c, err)
		}
		encoded, err := c.action.actionBytes()
		if err != nil || !bytes.Equal(encoded, []byte{2, byte(codes[i] & 255), byte((codes[i] >> 8) & 255)}) {
			t.Fatal("media byte order", name, encoded, err)
		}
		if !strings.Contains(mediaHelp, name) {
			t.Fatal("missing media name in help", name)
		}
	}
}

func TestPublicActionInvalidBeforeAccess(t *testing.T) {
	cases := [][]string{
		{"mouse", ""},
		{"mouse", "left", "right"},
		{"mouse", "left+left"},
		{"mouse", "none+left"},
		{"mouse", "left+none"},
		{"mouse", "wheelup+left"},
		{"mouse", "left+wheeldown"},
		{"mouse", "wheelup+wheeldown"},
		{"mouse", "0x01"},
		{"mouse", "wheelup", "--wheel=0"},
		{"mouse", "wheeldown", "--wheel=-1"},
		{"mouse", "left", "--x=0", "--x=1"},
		{"mouse", "--x=0"},
		{"media", ""},
		{"media", "0x0000"},
		{"media", "0x023d"},
		{"media", "0x00cd", "extra"},
		{"media", "0xcd"},
		{"media", "0x000cd"},
		{"media", "205"},
		{"media", "0x+001"},
		{"media", "0x00gg"},
		{"multi", ""},
		{"multi", "a,"},
		{"multi", ",a"},
		{"multi", "a,,b"},
		{"multi", "ctrl+a"},
		{"multi", "0x04"},
		{"multi", "lang3"},
		{"multi", "a", "b"},
		{"multi", strings.TrimSuffix(strings.Repeat("a,", 116), ",")},
		{"multi", "--repeat=1"},
		{"multi", "a", "--repeat=1", "--repeat=2"},
	}
	for _, flag := range []string{"--x", "--y", "--wheel"} {
		for _, value := range []string{"-128", "128", "999999999999999999999999", "1.5", "", "no"} {
			cases = append(cases, []string{"mouse", "none", flag, value})
		}
		cases = append(cases, []string{"mouse", "none", flag})
	}
	for _, flag := range []string{"--interval", "--repeat"} {
		for _, value := range []string{"0", "-1", "65536", "999999999999999999999999", "", "no"} {
			cases = append(cases, []string{"multi", "a", flag + "=" + value})
		}
	}
	cases = append(cases, []string{"multi", "a", "--repeat=256"})
	for _, command := range []string{"mouse", "media", "multi"} {
		for _, flag := range []string{"--yes", "--write", "--capture-root=/tmp", "--on=press", "--device=1"} {
			cases = append(cases, []string{command, "--help", flag})
		}
	}
	access := publicAccess{interactive: func(io.Reader) bool { t.Fatal("invalid input accessed runtime"); return false }}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			err := runPublic(args, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
			if err == nil || ExitCode(err) != 2 || out.Len() != 0 {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
		})
	}
	for _, command := range []string{"mouse", "media", "multi"} {
		var out bytes.Buffer
		if err := runPublic([]string{command, "--help"}, &cliRuntime{strings.NewReader(""), &out, io.Discard}, access); err != nil || !strings.Contains(out.String(), "Usage: wonkey "+command) {
			t.Fatal(out.String(), err)
		}
	}
}

func TestPublicActionQueries(t *testing.T) {
	for _, action := range actionExamples() {
		for _, command := range []string{"key", "mouse", "media", "multi", "rgb"} {
			t.Run(fmt.Sprintf("%T/%s", action, command), func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "absent")
				t.Setenv("XDG_STATE_HOME", root)
				transport := newSettingsTransport()
				transport.current = actionConfiguration(t, action)
				access := publicActionTestAccess(t, transport, nil)
				access.interactive = func(io.Reader) bool { return false }
				var out bytes.Buffer
				if err := runPublic([]string{command}, &cliRuntime{strings.NewReader(""), &out, io.Discard}, access); err != nil {
					t.Fatal(err)
				}
				if command == "rgb" {
					if !strings.Contains(out.String(), "Colour") || strings.Contains(out.String(), "  Action ") {
						t.Fatal(out.String())
					}
				} else if _, keyboard := action.(KeyboardAction); keyboard {
					if !strings.Contains(out.String(), "ctrl+shift+alt+super+lang2") || !strings.Contains(out.String(), "release") {
						t.Fatal(out.String())
					}
				} else if !strings.Contains(out.String(), actionDescription(action)) || strings.Contains(out.String(), "  Key ") {
					t.Fatal(out.String())
				}
				checkNoUpload(t, transport)
				if len(transport.packets) != 4 {
					t.Fatal("unexpected query count", len(transport.packets))
				}
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatal("query created backup storage", err)
				}
			})
		}
	}
}

func publicActionTestAccess(t *testing.T, first *settingsTransport, apply func(Candidate, string, ApplyTarget, configurationChanges, func(configuration, configuration, string) (bool, error)) (ApplyResult, error)) publicAccess {
	t.Helper()
	if apply == nil {
		apply = func(Candidate, string, ApplyTarget, configurationChanges, func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
			t.Fatal("unexpected apply")
			return ApplyResult{}, nil
		}
	}
	return publicAccess{
		interactive: func(io.Reader) bool { return true },
		discover:    func() ([]Candidate, error) { return []Candidate{{PhysicalPath: "synthetic", Compatible: true}}, nil },
		query: func(c Candidate) (CaptureResult, error) {
			q, err := queryCapture(first, true)
			q.result.PhysicalPath = c.PhysicalPath
			return q.result, err
		},
		apply: apply,
	}
}

func TestPublicActionWorkflow(t *testing.T) {
	for _, args := range [][]string{{"mouse", "none"}, {"mouse", "left+right", "--x=-127", "--y=127", "--wheel=-1"}, {"media", "bookmarks"}, {"multi", "lang2,f13,a", "--interval=65535", "--repeat=255"}} {
		for _, state := range []string{"save", "cancel", "eof", "nonterminal", "no-op", "drift", "became-no-op", "backup-failed", "upload-echo", "commit-echo", "deadline", "readback"} {
			t.Run(strings.Join(args, " ")+"/"+state, func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				first := newSettingsTransport()
				c, err := parsePublic(args)
				if err != nil {
					t.Fatal(err)
				}
				if state == "no-op" {
					first.current = actionConfiguration(t, c.action)
				}
				original := first.current
				fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
				fresh.current = original
				applied := false
				var out bytes.Buffer
				access := publicActionTestAccess(t, first, func(candidate Candidate, destination string, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					applied = true
					if destination != root || target != settingsTarget() || !strings.Contains(out.String(), " -> ") || !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
						t.Fatal("missing preview or wrong transaction target", out.String())
					}
					switch state {
					case "drift":
						fresh.current[123] ^= 1
					case "became-no-op":
						fresh.current, _ = changes.configuration(fresh.current)
					case "upload-echo":
						fresh.failAt, fresh.failure = 5, "echo"
					case "commit-echo":
						fresh.failAt, fresh.failure = 8, "echo"
					case "deadline":
						fresh.failAt, fresh.failure = 5, "read-deadline"
					case "readback":
						fresh.mismatch = true
					}
					dir, err := newCapture(destination, candidate)
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					if state == "backup-failed" {
						if err := saveExclusive(dir, "reply-01.bin", []byte("occupied")); err != nil {
							t.Fatal(err)
						}
					}
					return applySettingsConfirmed(fresh, dir, target, changes, true, time.Second, guard, true)
				})
				input := "\n"
				if state == "cancel" {
					input = "n\n"
				}
				if state == "eof" {
					input = ""
				}
				if state == "nonterminal" {
					access.interactive = func(io.Reader) bool { return false }
					access.discover = func() ([]Candidate, error) { t.Fatal("nonterminal discovered devices"); return nil, nil }
				}
				err = runPublic(args, &cliRuntime{strings.NewReader(input), &out, io.Discard}, access)
				noApply := state == "cancel" || state == "eof" || state == "nonterminal" || state == "no-op"
				wantError := !noApply && state != "save" || state == "nonterminal"
				if (err != nil) != wantError || applied == noApply {
					t.Fatalf("error=%v applied=%t output=%s", err, applied, out.String())
				}
				checkNoUpload(t, first)
				if noApply || state == "drift" || state == "became-no-op" || state == "backup-failed" {
					checkNoUpload(t, fresh.settingsTransport)
				}
				if noApply {
					entries, err := os.ReadDir(root)
					if err != nil || len(entries) != 0 {
						t.Fatal("created backup without approval", entries, err)
					}
				}
				if state == "save" {
					want, err := (ActionChanges{Action: c.action}).configuration(original)
					if err != nil || fresh.current != want || !fresh.checked || len(fresh.packets) != 11 || !strings.Contains(out.String(), "Settings saved. All 128 readback bytes match.") {
						t.Fatal("wrong saved configuration or missing durable backup", err, out.String())
					}
					_, backup, err := loadCapture(fresh.dir)
					if err != nil || backup != original {
						t.Fatal("backup does not match original", err)
					}
				}
			})
		}
	}
}

func TestPublicTypedLayoutReplacement(t *testing.T) {
	for _, action := range actionExamples()[1:] {
		for _, args := range [][]string{{"key", "ctrl+f13"}, {"key", "f13", "--on=release"}, {"rgb", "off"}, {"rgb", "static", "123456"}} {
			t.Run(fmt.Sprintf("%T/%s", action, strings.Join(args, " ")), func(t *testing.T) {
				publicTestCaptureRoot(t)
				first := newSettingsTransport()
				first.current = actionConfiguration(t, action)
				original := first.current
				fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
				fresh.current = original
				access := publicActionTestAccess(t, first, func(candidate Candidate, root string, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
					dir, err := newCapture(root, candidate)
					if err != nil {
						return ApplyResult{}, err
					}
					fresh.dir = dir
					return applySettingsConfirmed(fresh, dir, target, changes, true, time.Second, guard, true)
				})
				var out bytes.Buffer
				if err := runPublic(args, &cliRuntime{strings.NewReader("yes\n"), &out, io.Discard}, access); err != nil || !fresh.checked {
					t.Fatal(err, out.String())
				}
				if args[0] == "key" {
					want := KeyboardAction{Trigger: 1, Modifiers: 1, Key: 0x68}
					if len(args) == 3 {
						want.Trigger, want.Modifiers = 2, 0
					}
					got, err := decodeAction(fresh.current)
					if err != nil || got != want || !bytes.Equal(original[124:], fresh.current[124:]) {
						t.Fatal(got, err)
					}
				} else if !bytes.Equal(original[:124], fresh.current[:124]) {
					t.Fatal("RGB changed action or unknown bytes")
				}
			})
		}
	}
}

func TestPublicActionOutput(t *testing.T) {
	for _, tc := range []struct {
		action Action
		want   string
	}{
		{MouseAction{}, "mouse none (x=0, y=0, wheel=0)"},
		{MouseAction{Buttons: 3, X: -127, Y: 127, Wheel: -1}, "mouse left+right (x=-127, y=127, wheel=-1)"},
		{MediaAction{Usage: 0x00b6}, "media prev (0x00b6)"},
		{MediaAction{Usage: 0x0001}, "media 0x0001"},
		{MultiAction{Interval: 50, Repeat: 1, Keys: []byte{4, 0x68}}, "multi a,f13 (interval=50 ms, repeat=1)"},
	} {
		if got := actionDescription(tc.action); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
		for _, needle := range []string{"Current", "  Action", " -> ", "Save settings?"} {
			t.Run(tc.want+"/"+needle, func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				first := newSettingsTransport()
				first.current = actionConfiguration(t, tc.action)
				access := publicActionTestAccess(t, first, nil)
				out := &failingHumanOutput{needle: needle}
				err := runPublic([]string{"key", "f13"}, &cliRuntime{strings.NewReader("yes\n"), out, io.Discard}, access)
				if err == nil || !out.failed {
					t.Fatal("output failure was not returned", err)
				}
				checkNoUpload(t, first)
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal("output failure created a backup", err)
				}
			})
		}
	}
}

func TestPublicTypedRestoreWithoutSourcesAndAmbiguousTrigger(t *testing.T) {
	for _, action := range actionExamples()[1:] {
		for _, args := range [][]string{{"restore"}, {"restore", "absent-source"}, {"key", "f13", "--on=both"}} {
			t.Run(fmt.Sprintf("%T/%v", action, args), func(t *testing.T) {
				root := publicTestCaptureRoot(t)
				first := newSettingsTransport()
				first.current = actionConfiguration(t, action)
				access := publicActionTestAccess(t, first, nil)
				input := strings.NewReader("yes\n")
				err := runPublic(args, &cliRuntime{input, io.Discard, io.Discard}, access)
				if (err != nil) != (len(args) > 1) || input.Len() != len("yes\n") {
					t.Fatal("missing source or ambiguous conversion reached confirmation", err)
				}
				checkNoUpload(t, first)
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal("blocked action created a backup", err)
				}
			})
		}
	}
}

func TestPublicUnsupportedTypedLayouts(t *testing.T) {
	for _, raw := range [][]byte{{1, 8, 0, 0, 0}, {1, 0, 128, 0, 0}, {2, 0, 0}, {2, 0x3d, 2}, {3, 0, 0, 1, 1, 4}, {3, 0, 1, 0, 1, 4}, {3, 0, 1, 1, 116}} {
		for _, args := range [][]string{{"key"}, {"mouse"}, {"media"}, {"multi"}, {"rgb"}, {"mouse", "left"}, {"media", "play"}, {"multi", "a"}, {"rgb", "off"}, {"key", "a"}} {
			t.Run(fmt.Sprintf("%x/%v", raw, args), func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "absent")
				t.Setenv("XDG_STATE_HOME", root)
				first := newSettingsTransport()
				copy(first.current[:], raw)
				access := publicActionTestAccess(t, first, nil)
				var out bytes.Buffer
				err := runPublic(args, &cliRuntime{strings.NewReader("yes\n"), &out, io.Discard}, access)
				if len(args) == 1 {
					if err != nil || !strings.Contains(out.String(), fmt.Sprintf("0x%02x (byte 0)", raw[0])) || !strings.Contains(out.String(), fmt.Sprintf("%x", first.current[:16])) || !strings.Contains(out.String(), "Fields are not decoded.") {
						t.Fatal(err, out.String())
					}
				} else if err == nil {
					t.Fatal("unsupported layout accepted")
				}
				checkNoUpload(t, first)
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatal("unsupported layout created backup", err)
				}
			})
		}
	}
}
