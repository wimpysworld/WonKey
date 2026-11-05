package xfkey

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"wonkey/internal/ch57"
)

func checkPublicHeader(t *testing.T, text string) {
	t.Helper()
	if !strings.HasPrefix(text, "1️⃣ WonKey\n\n") || strings.Count(text, "WonKey\n") != 1 {
		t.Fatalf("expected one header on the first line: %q", text)
	}
}

func TestPublicHeaderOutputFailure(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"key", "--help"}, {"key"}, {"key", "f13"}, {"invalid"}} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/short=%v", args, short), func(t *testing.T) {
				failed := &failingHumanOutput{short: short}
				var other bytes.Buffer
				var out, diagnostic io.Writer = failed, &other
				code := 1
				if len(args) == 1 && (args[0] == "invalid" || args[0] == "key") {
					out, diagnostic, code = &other, failed, 2
				}
				access := publicAccess{
					interactive: func(io.Reader) bool { t.Fatal("header failure accessed runtime"); return false },
					discover:    func() ([]Candidate, error) { t.Fatal("header failure accessed devices"); return nil, nil },
				}
				err := runPublic(args, &cliRuntime{strings.NewReader(""), out, diagnostic}, access)
				want := io.ErrClosedPipe
				if short {
					want = io.ErrShortWrite
				}
				if !errors.Is(err, want) || ExitCode(err) != code || failed.calls != 1 || other.Len() != 0 {
					t.Fatalf("error=%v writes=%d other=%q", err, failed.calls, other.String())
				}
			})
		}
	}
}

func checkPublicOutputFailure(t *testing.T, args []string, input, needle string, multiple, short bool) {
	t.Helper()
	first, fresh := newSettingsTransport(), newSettingsTransport()
	queried, applied := false, false
	out := &failingHumanOutput{needle: needle, short: short}
	access := publicAccess{
		interactive: func(io.Reader) bool { return true },
		discover: func() ([]Candidate, error) {
			candidates := []Candidate{{PhysicalPath: "1-2", Compatible: true}}
			if multiple {
				candidates = append(candidates, Candidate{PhysicalPath: "1-3", Compatible: true})
			}
			return candidates, nil
		},
		query: func(c Candidate) (CaptureResult, error) {
			queried = true
			q, err := queryCapture(first, true)
			q.result.PhysicalPath = c.PhysicalPath
			return q.result, err
		},
		apply: func(_ Candidate, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration) (bool, error)) (ApplyResult, error) {
			applied = true
			return applySettingsConfirmed(fresh, target, changes, true, time.Second, guard, true)
		},
	}
	reader := strings.NewReader(input)
	err := runPublic(args, &cliRuntime{reader, out, io.Discard}, access)
	if reader.Len() != len(input) {
		t.Fatal("input read after output failure")
	}
	want := io.ErrClosedPipe
	if short {
		want = io.ErrShortWrite
	}
	if !out.failed || !errors.Is(err, want) || applied || queried != (needle != "") {
		t.Fatalf("output failed=%v error=%v applied=%v queried=%v", out.failed, err, applied, queried)
	}
	checkNoUpload(t, first)
	checkNoUpload(t, fresh)
}

func TestPublicOutputFailure(t *testing.T) {
	for _, short := range []bool{false, true} {
		for _, tc := range []struct {
			name, needle string
			args         []string
			multiple     bool
		}{
			{"all-output", "", []string{"key", "f13"}, false},
			{"second-device-heading", "[2] XFKEY One Key Max (0112)", []string{"key", "f13", "--device", "2"}, true},
			{"second-device-path", "1-3", []string{"key", "f13", "--device=2"}, true},
			{"preview-heading", "[1] XFKEY One Key Max (0112)", []string{"key", "f13"}, false},
			{"preview-field", "USB path", []string{"key", "f13"}, false},
			{"preview-colour", "Colour", []string{"rgb", "off"}, false},
			{"changes-heading", "Changes:", []string{"key", "f13"}, false},
			{"changes-row", " -> ", []string{"key", "f13"}, false},
			{"confirmation", "Save settings?", []string{"key", "f13"}, false},
		} {
			t.Run(fmt.Sprintf("%s/short=%v", tc.name, short), func(t *testing.T) {
				checkPublicOutputFailure(t, tc.args, "\n", tc.needle, tc.multiple, short)
			})
		}
	}
}

func TestPublicParser(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		want       Changes
		wantDevice int
	}{
		{nil, nil, 0},
		{[]string{}, nil, 0},
		{[]string{"key", "f13"}, Changes{"key": 0x68, "modifiers": 0}, 0},
		{[]string{"key", "ctrl+shift+f13", "--device", "1"}, Changes{"key": 0x68, "modifiers": 3}, 1},
		{[]string{"key", "--device=2", "alt+super+enter", "--on", "release"}, Changes{"key": 0x28, "modifiers": 12, "trigger": 2}, 2},
		{[]string{"key", "SUPER+f13"}, Changes{"key": 0x68, "modifiers": 8}, 0},
		{[]string{"key", "--on=both", "f13"}, Changes{"key": 0x68, "modifiers": 0, "trigger": 3}, 0},
		{[]string{"key", "f13", "--on", "press"}, Changes{"key": 0x68, "modifiers": 0, "trigger": 1}, 0},
		{[]string{"rgb", "static", "0000ff", "--device", "3"}, Changes{"rgb-mode": 1, "red": 0, "green": 0, "blue": 255}, 3},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := parsePublic(tc.args)
			if err != nil || !reflect.DeepEqual(got.changes, tc.want) || got.device != tc.wantDevice {
				t.Fatal(got, err)
			}
		})
	}
	for name, mode := range lightingValues {
		c, err := parsePublic([]string{"rgb", strings.ToUpper(name)})
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

func TestPublicRGBColourArguments(t *testing.T) {
	for _, mode := range []string{"cycle-slow", "cycle-fast", "off"} {
		for _, spelling := range []string{mode, strings.ToUpper(mode)} {
			for _, colour := range []string{"1234ab", "", "#1234ab", "12345", "gggggg"} {
				t.Run(spelling+"/"+colour, func(t *testing.T) {
					access := publicAccess{
						interactive: func(io.Reader) bool { t.Fatal("invalid colour accessed runtime"); return false },
						discover:    func() ([]Candidate, error) { t.Fatal("invalid colour accessed devices"); return nil, nil },
					}
					var diagnostic bytes.Buffer
					err := runPublic([]string{"rgb", spelling, colour}, &cliRuntime{strings.NewReader(""), io.Discard, &diagnostic}, access)
					want := fmt.Sprintf("RGB mode %q does not use a colour; omit RGB", mode)
					if err == nil || ExitCode(err) != 2 || !strings.Contains(diagnostic.String(), want) {
						t.Fatalf("error=%v output=%q", err, diagnostic.String())
					}
				})
			}
		}
	}
	for _, mode := range []string{"static", "breathe", "flash", "held", "toggle"} {
		for _, spelling := range []string{mode, strings.ToUpper(mode)} {
			t.Run(spelling, func(t *testing.T) {
				c, err := parsePublic([]string{"rgb", spelling, "12AbEF"})
				want := Changes{"rgb-mode": lightingValues[mode], "red": 0x12, "green": 0xab, "blue": 0xef}
				if err != nil || !reflect.DeepEqual(c.changes, want) {
					t.Fatal(c, err)
				}
			})
		}
	}
}

func TestPublicRGBHelpArguments(t *testing.T) {
	for _, mode := range []string{"cycle-slow", "cycle-fast", "off"} {
		if !strings.Contains(rgbHelp, "  "+mode+" ") || strings.Contains(rgbHelp, mode+" [RGB]") {
			t.Fatal("unexpected colour argument for", mode)
		}
	}
	for _, mode := range []string{"static", "breathe", "flash", "held", "toggle"} {
		if !strings.Contains(rgbHelp, mode+" [RGB]") {
			t.Fatal("missing optional colour for", mode)
		}
	}
	for _, text := range []string{"six hexadecimal digits without #", "Omit RGB to preserve the current colour"} {
		if !strings.Contains(rgbHelp, text) {
			t.Fatal("missing colour guidance", text)
		}
	}
}

func TestPublicRefusesInvalidBeforeAccess(t *testing.T) {
	cases := [][]string{
		{"key"},
		{"mouse"},
		{"media"},
		{"multi"},
		{"rgb"},
		{"rgb", "steady"},
		{"rgb", "flowing"},
		{"rgb", "gradient"},
		{"rgb", "neon"},
		{"rgb", "STEADY", "0000ff"},
		{"rgb", "FLOWING", "0000ff"},
		{"rgb", "GRADIENT", "gggggg"},
		{"rgb", "NEON", "#0000ff"},
		{"key", "gui+f13"},
		{"key", "GUI+enter"},
		{"key", "super+super+f13"},
		{"restore"},
		{"restore", "one"},
		{"restore", "one", "two"},
		{"restore", "--yes"},
		{"restore", "--capture-root", "/tmp"},
		{"restore", "--write"},
		{"restore", "--help"},
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
		{"key", "!"},
		{"key", "rightctrl+left"},
		{"key", "0x04"},
		{"key", "a+b"},
		{"key", ""},
		{"key", "f13+"},
		{"key", "+f13"},
		{"key", "ctrl+ctrl+f13"},
		{"key", "none+f13"},
		{"key", "f13+ctrl"},
		{"key", "f13", "enter"},
		{"rgb", ""},
		{"rgb", "blue"},
		{"rgb", "static", "#0000ff"},
		{"rgb", "static", "00000"},
		{"rgb", "static", "gggggg"},
		{"rgb", "off", "000000", "extra"},
		{"--help", "--yes"},
		{"key", "--report"},
		{"key", "--report="},
		{"key", "--report=output"},
		{"key", "--report", "output"},
		{"key", "--report=output", "--help"},
		{"mouse", "--report=output"},
		{"media", "--report=output"},
		{"multi", "--report=output"},
		{"rgb", "--report=output"},
		{"key", "--device"},
		{"key", "--device="},
		{"--device", "1"},
		{"--device=1"},
		{"key", "f13", "--device", "0"},
		{"key", "f13", "--device=0"},
		{"key", "f13", "--device", "-1"},
		{"key", "f13", "--device", "+1"},
		{"key", "f13", "--device", "01"},
		{"key", "f13", "--device", "abc"},
		{"key", "f13", "--device", "1", "--device", "2"},
		{"key", "f13", "--device=1", "--device=1"},
		{"key", "--device", "1"},
		{"mouse", "--device", "1"},
		{"media", "--device", "1"},
		{"multi", "--device", "1"},
		{"rgb", "--device", "1"},
		{"key", "--device", "1189:8891"},
		{"key", "--device=0x1189:0x8890"},
		{"key", "--device", "1189:8890", "--device=1189:8890"},
		{"key", "--device=1189:8890", "--device", "1189:8890"},
		{"key", "--device=1189:8890", "f13"},
		{"key", "f13", "--device", "1189:8890"},
		{"key", "--device=1189:8890", "--on", "press"},
		{"key", "--on=press", "--device=1189:8890"},
		{"key", "--device=1189:8890", "--help", "f13"},
		{"key", "--device=1189:8890", "--write"},
		{"key", "--device=1189:8890", "--x=1"},
		{"mouse", "--device=1189:8890"},
		{"media", "--device=1189:8890"},
		{"multi", "--device=1189:8890"},
		{"rgb", "--device=1189:8890"},
		{"restore", "--device=1189:8890"},
	}
	for _, flag := range []string{"--device", "--report", "--capture-root", "--json", "--yes", "--dry-run", "--write", "--target", "--path", "--key", "--modifiers", "--trigger", "--lighting", "--colour", "--rgb-mode", "--red", "--expect-identifier", "--expect-version", "--json=false", "--yes=false"} {
		cases = append(cases, []string{"key", "f13", flag}, []string{"rgb", "off", flag}, []string{"key", "--help", flag})
	}
	access := publicAccess{discover: func() ([]Candidate, error) { t.Fatal("discovery on invalid input"); return nil, nil }, interactive: func(io.Reader) bool { t.Fatal("runtime on invalid input"); return false }}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			err := runPublic(args, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
			checkPublicHeader(t, diagnostic.String())
			if err == nil || ExitCode(err) != 2 || out.Len() != 0 || diagnostic.Len() == 0 {
				t.Fatal(err, out.String(), diagnostic.String())
			}
		})
	}
	for command, want := range map[string]string{
		"key":   `key requires a key combination, for example "wonkey key f13"; run "wonkey key --help" for supported keys`,
		"mouse": `mouse requires buttons or wheel direction, for example "wonkey mouse left"; run "wonkey mouse --help" for options`,
		"media": `media requires a media action, for example "wonkey media volumeup"; run "wonkey media --help" for actions`,
		"multi": `multi requires a comma-separated key list, for example "wonkey multi ctrl,c"; run "wonkey multi --help" for options`,
		"rgb":   `rgb requires a lighting mode, for example "wonkey rgb static 0000ff" or "wonkey rgb off"; run "wonkey rgb --help" for modes`,
	} {
		var out, diagnostic bytes.Buffer
		err := runPublic([]string{command}, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
		if err == nil || ExitCode(err) != 2 || out.Len() != 0 || err.Error() != want || !strings.Contains(strings.ReplaceAll(diagnostic.String(), "\n", " "), want) {
			t.Fatalf("%s: err=%v diagnostic=%q", command, err, diagnostic.String())
		}
	}
}

func TestPublicHelpNoAccess(t *testing.T) {
	access := publicAccess{interactive: func(io.Reader) bool { t.Fatal("help accessed runtime"); return false }}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"key", "--help"}, {"mouse", "--help"}, {"media", "--help"}, {"multi", "--help"}, {"rgb", "--help"}, {"key", "f13", "--help"}, {"rgb", "off", "-h"}} {
		var out bytes.Buffer
		if err := runPublic(args, &cliRuntime{strings.NewReader(""), &out, io.Discard}, access); err != nil {
			t.Fatal(err)
		}
		checkPublicHeader(t, out.String())
		if strings.Contains(out.String(), "One key, to rule them all.") {
			t.Fatal(out.String())
		}
		if (args[0] == "--help" || args[0] == "-h") && !strings.HasPrefix(out.String(), "1️⃣ WonKey\n\nInspect and configure supported macropads.\n") {
			t.Fatal(out.String())
		}
		for _, removed := range []string{"--report", "--json", "--yes", "--dry-run", "advanced", "wonkey set", "Apply result", "restore"} {
			if strings.Contains(out.String(), removed) {
				t.Fatal(removed, out.String())
			}
		}
		if !strings.Contains(out.String(), "Usage:") || !strings.Contains(out.String(), "--device N") || strings.Contains(out.String(), "\x1b") {
			t.Fatal(out.String())
		}
	}
}

func TestPublicWorkflow(t *testing.T) {
	for _, name := range []string{"confirm", "confirm-blank", "confirm-upper", "confirm-yes", "confirm-yes-upper", "cancel-upper", "cancel-no", "cancel-no-upper", "old-confirm", "space-only", "rgb-write", "rgb-mode-cycle-slow", "rgb-mode-cycle-fast", "rgb-mode-off", "clear-modifiers", "pasted-crlf", "first-device", "second-device", "multiple-missing-device", "query", "query-multiple", "zero", "incompatible", "cancel", "eof", "partial-write", "invalid-confirm", "space-confirm", "out-of-range", "no-op", "noninteractive", "identity-changed", "version-changed", "settings-changed", "became-no-op", "mismatch", "echo", "deadline", "unsupported", "query-failed", "discover-failed", "wrong-path", "wrong-model", "missing-settings", "short-settings", "bad-settings"} {
		t.Run(name, func(t *testing.T) {
			args := []string{"key", "f13"}
			if name == "rgb-write" {
				args = []string{"rgb", "static", "0000ff"}
			}
			if mode, ok := strings.CutPrefix(name, "rgb-mode-"); ok {
				args = []string{"rgb", strings.ToUpper(mode)}
			}
			if (name == "query" || name == "query-multiple") && name != "query-failed" {
				args = nil
			}
			if name == "no-op" {
				args = []string{"key", "enter"}
			}
			multiple := name == "first-device" || name == "second-device" || name == "multiple-missing-device" || name == "query-multiple" || name == "out-of-range"
			switch name {
			case "first-device":
				args = append(args, "--device", "1")
			case "second-device":
				args = append(args, "--device=2")
			case "out-of-range":
				args = append(args, "--device", "3")
			}
			input := "y\n"
			switch name {
			case "pasted-crlf":
				input = "y\r\n"
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
			case "eof":
				input = ""
			case "partial-write":
				input = "yes"
			case "invalid-confirm":
				input = "invalid\ny\n"
			case "space-confirm":
				input = " y\n"
			case "query-multiple":
				input = ""
			}
			first := newSettingsTransport()
			if strings.HasPrefix(name, "rgb-mode-") {
				first.current[124] = 2
			}
			if name == "unsupported" {
				first.current[3] = 2
			}
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			if strings.HasPrefix(name, "rgb-mode-") {
				fresh.current = first.current
			}
			if name == "clear-modifiers" {
				first.current[2], fresh.current[2] = 15, 15
			}
			var out, diagnostic bytes.Buffer
			queried, applied, discovered := false, false, false
			var queriedPaths []string
			wantPath := "1-1.2"
			if name == "second-device" {
				wantPath = "1-1.3"
			}
			access := publicAccess{
				interactive: func(io.Reader) bool {
					return name != "noninteractive" && name != "query" && name != "query-multiple"
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
					queriedPaths = append(queriedPaths, c.PhysicalPath)
					if name != "query-multiple" && c.PhysicalPath != wantPath {
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
				apply: func(c Candidate, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration) (bool, error)) (ApplyResult, error) {
					applied = true
					if c.PhysicalPath != wantPath || target != settingsTarget() || !strings.Contains(out.String(), " -> ") || !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
						t.Fatal(c, target, out.String())
					}
					switch name {
					case "identity-changed":
						fresh.identity[6] ^= 1
					case "version-changed":
						fresh.identity[4] ^= 1
					case "settings-changed":
						fresh.current[63] ^= 1
					case "became-no-op":
						fresh.current, _ = changes.configuration(fresh.current)
					case "mismatch":
						fresh.mismatch = true
					case "echo":
						fresh.failAt, fresh.failure = 5, "echo"
					case "deadline":
						fresh.failAt, fresh.failure = 5, "read-deadline"
					}
					return applySettingsConfirmed(fresh, target, changes, true, time.Second, guard, true)
				},
			}
			err := runPublic(args, &cliRuntime{strings.NewReader(input), &out, &diagnostic}, access)
			checkPublicHeader(t, out.String())
			if strings.Contains(diagnostic.String(), "WonKey") {
				t.Fatal("runtime error repeated the header", diagnostic.String())
			}
			failure := name == "zero" || name == "incompatible" || name == "multiple-missing-device" || name == "out-of-range" || strings.HasPrefix(name, "noninteractive") || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "mismatch" || name == "echo" || name == "deadline" || name == "unsupported" || strings.HasSuffix(name, "failed") || strings.HasPrefix(name, "wrong-") || strings.HasSuffix(name, "settings")
			if (err != nil) != failure {
				t.Fatal(err, out.String(), diagnostic.String())
			}
			success := name == "rgb-write" || strings.HasPrefix(name, "rgb-mode-") || name == "clear-modifiers" || name == "pasted-crlf" || strings.HasPrefix(name, "confirm") || name == "first-device" || name == "second-device"
			wantApply := success || strings.HasSuffix(name, "changed") || name == "became-no-op" || name == "mismatch" || name == "echo" || name == "deadline"
			if applied != wantApply {
				t.Fatal("apply", applied, err)
			}
			noQuery := name == "zero" || name == "incompatible" || name == "multiple-missing-device" || name == "out-of-range" || strings.HasPrefix(name, "noninteractive") || name == "discover-failed"
			if queried == noQuery {
				t.Fatal("query", queried, err)
			}
			if name == "multiple-missing-device" {
				for _, want := range []string{"multiple compatible devices connected; specify --device <N>:", "--device 1  XFKEY One Key Max (0112) (USB path 1-1.2)", "--device 2  XFKEY One Key Max (0112) (USB path 1-1.3)"} {
					if !strings.Contains(diagnostic.String(), want) {
						t.Fatalf("missing %q in %q", want, diagnostic.String())
					}
				}
			}
			if name == "out-of-range" && !strings.Contains(diagnostic.String(), "device 3 not found (2 compatible device(s) connected)") {
				t.Fatalf("unexpected out-of-range diagnostic: %q", diagnostic.String())
			}
			if name == "second-device" && !strings.Contains(out.String(), "[2] XFKEY One Key Max (0112)\n") {
				t.Fatalf("missing [2] heading in %q", out.String())
			}
			if name == "query-multiple" {
				if !reflect.DeepEqual(queriedPaths, []string{"1-1.2", "1-1.3"}) || !strings.Contains(out.String(), "[1] XFKEY One Key Max (0112)\n") || !strings.Contains(out.String(), "[2] XFKEY One Key Max (0112)\n") {
					t.Fatal("query-multiple did not enumerate all compatible devices", queriedPaths, out.String())
				}
			}
			if strings.HasPrefix(name, "noninteractive") && discovered {
				t.Fatal("noninteractive write discovered devices")
			}
			checkNoUpload(t, first)
			if !wantApply || strings.HasSuffix(name, "changed") || name == "became-no-op" {
				checkNoUpload(t, fresh.settingsTransport)
			}
			if success {
				if !strings.Contains(out.String(), "Save settings? [Y/n]: ") {
					t.Fatal("confirmation prompt missing", out.String())
				}
				if !fresh.checked || len(fresh.packets) != 11 {
					t.Fatal("fresh query/write missing")
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
			if mode, ok := strings.CutPrefix(name, "rgb-mode-"); ok {
				want := first.current
				want[124] = map[string]byte{"cycle-slow": 1, "cycle-fast": 5, "off": 6}[mode]
				if fresh.current != want {
					t.Fatal("mode-only command changed colour or unrelated bytes")
				}
			}
			if name == "query" || name == "query-multiple" {
				for _, field := range []string{"\n  Key ", "\n  On ", "\n  Mode ", "\n  Colour "} {
					if !strings.Contains(out.String(), field) {
						t.Fatal("missing field", field, out.String())
					}
				}
			}
			for _, token := range []string{"wonkey-target-v1:", "Apply result", "Target", "Warning: this changes stored settings.", "Persistence after reconnect"} {
				if strings.Contains(out.String(), token) {
					t.Fatal(out.String())
				}
			}
		})
	}
}

func ch57TestAccess(t *testing.T) publicAccess {
	t.Helper()
	return publicAccess{
		interactive: func(io.Reader) bool { return true },
		discover:    func() ([]Candidate, error) { return nil, nil },
		query: func(Candidate) (CaptureResult, error) {
			t.Fatal("CH57 probe reached XFKEY query")
			return CaptureResult{}, nil
		},
		apply: func(Candidate, ApplyTarget, configurationChanges, func(configuration, configuration) (bool, error)) (ApplyResult, error) {
			t.Fatal("CH57 probe reached apply")
			return ApplyResult{}, nil
		},
	}
}

func TestPublicCH57Selection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		count      int
		shortReply bool
		failure    bool
	}{
		{"single", 1, false, false},
		{"single-short-aa", 1, true, false},
		{"multiple", 2, false, false},
		{"multiple-short-aa", 2, true, false},
		{"none", 0, false, true},
		{"discovery-error", 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "absent")
			t.Setenv("XDG_STATE_HOME", root)
			access := ch57TestAccess(t)
			access.interactive = func(io.Reader) bool { return false }
			candidates := []ch57.DiscoveredCandidate{
				{Candidate: ch57.Candidate{PhysicalPath: "1-2"}, BusNumber: 1, DeviceNumber: 19},
				{Candidate: ch57.Candidate{PhysicalPath: "1-3"}, BusNumber: 1, DeviceNumber: 20},
			}
			discovered := 0
			var probedPaths []string
			access.discoverCH57 = func(sysRoot string) ([]ch57.DiscoveredCandidate, error) {
				discovered++
				if sysRoot != "/sys/bus/usb/devices" {
					t.Fatal(sysRoot)
				}
				if tc.name == "discovery-error" {
					return nil, errors.New("synthetic discovery failure")
				}
				return candidates[:tc.count], nil
			}
			access.probeCH57 = func(sysRoot, devRoot string, selected ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
				probedPaths = append(probedPaths, selected.PhysicalPath)
				if sysRoot != "/sys/bus/usb/devices" || devRoot != "/dev/bus/usb" {
					t.Fatal("wrong probe roots", selected, sysRoot, devRoot)
				}
				for _, candidate := range candidates {
					if candidate.PhysicalPath == selected.PhysicalPath && candidate != selected {
						t.Fatal("candidate binding changed")
					}
				}
				raw := append([]byte{3}, bytes.Repeat([]byte{0xab}, 64)...)
				if tc.shortReply {
					raw = []byte{0xaa}
				}
				return ch57.ProbeResult{Status: ch57.ProbeRawReport, Raw: raw, Count: len(raw), ReportDescriptorVerified: true}, nil
			}
			command, err := parsePublic(nil)
			if err != nil || len(command.changes) != 0 || command.action != nil || command.help || command.name != "" {
				t.Fatal(command, err)
			}
			var out, diagnostic bytes.Buffer
			err = runPublic(nil, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
			if (err != nil) != tc.failure || discovered != 1 || len(probedPaths) != tc.count {
				t.Fatalf("error=%v discovered=%d probed=%v output=%q", err, discovered, probedPaths, out.String())
			}
			if tc.count > 0 {
				for i, candidate := range candidates[:tc.count] {
					wantTexts := []string{fmt.Sprintf("[%d] CH57x Macropad (1189:8890)", i+1), candidate.PhysicalPath, "3-key / 1-knob up to 12-key / 3-knob", "Not readable over USB"}
					if tc.shortReply {
						wantTexts = append(wantTexts, "Connected (write-only firmware)")
					} else {
						wantTexts = append(wantTexts, "Connected", "03"+strings.Repeat("ab", 64))
					}
					for _, text := range wantTexts {
						if !strings.Contains(out.String(), text) {
							t.Fatal("missing output", text, out.String())
						}
					}
				}
				if tc.shortReply {
					for _, unwanted := range []string{"Descriptor bytes", "Descriptor hex", "Input bytes", "Input hex", "Raw input is not decoded settings"} {
						if strings.Contains(out.String(), unwanted) {
							t.Fatal("unexpected raw probe diagnostic on short 0xaa reply", unwanted, out.String())
						}
					}
				}
				if strings.Count(out.String(), "CH57x Macropad (1189:8890)\n") != tc.count {
					t.Fatal("unexpected probe section count", out.String())
				}
			}
			for _, text := range []string{"Select a device", "Device number [cancel]: ", "Settings saved", "Save settings?"} {
				if strings.Contains(out.String(), text) {
					t.Fatal("unexpected prompt or write claim", text, out.String())
				}
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("probe created backup storage", err)
			}
		})
	}
}

func TestPublicUnifiedDiscoveryAndGuards(t *testing.T) {
	t.Run("mixed-read-enumerates-all", func(t *testing.T) {
		queried, probed := 0, 0
		transport := newSettingsTransport()
		access := publicAccess{
			interactive: func(io.Reader) bool { return false },
			discover: func() ([]Candidate, error) {
				return []Candidate{{PhysicalPath: "1-1", Compatible: true, VendorNode: NodeMetadata{Path: "/dev/hidraw0"}}}, nil
			},
			discoverCH57: func(string) ([]ch57.DiscoveredCandidate, error) {
				return []ch57.DiscoveredCandidate{{Candidate: ch57.Candidate{PhysicalPath: "1-2"}, BusNumber: 1, DeviceNumber: 19}}, nil
			},
			query: func(c Candidate) (CaptureResult, error) {
				queried++
				if c.PhysicalPath != "1-1" {
					t.Fatal(c)
				}
				q, err := queryCapture(transport, true)
				q.result.PhysicalPath = c.PhysicalPath
				return q.result, err
			},
			probeCH57: func(_ string, _ string, selected ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
				probed++
				if selected.PhysicalPath != "1-2" {
					t.Fatal(selected)
				}
				return ch57.ProbeResult{Status: ch57.ProbeRawReport, Raw: []byte{0xaa}, Count: 1}, nil
			},
		}
		var out bytes.Buffer
		err := runPublic(nil, &cliRuntime{strings.NewReader(""), &out, io.Discard}, access)
		if err != nil || queried != 1 || probed != 1 {
			t.Fatalf("err=%v queried=%d probed=%d out=%q", err, queried, probed, out.String())
		}
		checkPublicHeader(t, out.String())
		for _, text := range []string{"[1] XFKEY One Key Max (0112)", "1-1", "Key        enter", "On         press", "Mode       cycle-slow", "Colour     #FFFFFF", "[2] CH57x Macropad (1189:8890)", "3-key / 1-knob up to 12-key / 3-knob", "Connected (write-only firmware)", "Not readable over USB", "1-2"} {
			if !strings.Contains(out.String(), text) {
				t.Fatal("missing mixed read entry", text, out.String())
			}
		}
		if strings.Contains(out.String(), "Select a device") {
			t.Fatal("unexpected selection prompt on bare read", out.String())
		}
	})
	t.Run("mixed-write-selection", func(t *testing.T) {
		for _, tc := range []struct {
			args          []string
			wantXFKEY     bool
			wantErr       bool
			wantDiag      string
			wantOutSubstr string
		}{
			{[]string{"key", "f13", "--device", "1"}, true, false, "", "[1] XFKEY One Key Max (0112)"},
			{[]string{"key", "f13", "--device", "2"}, false, true, "live writes are not supported for 1189:8890", ""},
			{[]string{"key", "f13"}, false, true, "--device 1  XFKEY One Key Max (0112) (USB path 1-1)\n  --device 2  CH57x Macropad (1189:8890) (USB path 1-2)", ""},
			{[]string{"key", "f13", "--device", "3"}, false, true, "device 3 not found (2 compatible device(s) connected)", ""},
		} {
			queried, applied := false, false
			first := newSettingsTransport()
			fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
			access := publicAccess{
				interactive: func(io.Reader) bool { return true },
				discover: func() ([]Candidate, error) {
					return []Candidate{{PhysicalPath: "1-1", Compatible: true, VendorNode: NodeMetadata{Path: "/dev/hidraw0"}}}, nil
				},
				discoverCH57: func(string) ([]ch57.DiscoveredCandidate, error) {
					return []ch57.DiscoveredCandidate{{Candidate: ch57.Candidate{PhysicalPath: "1-2"}, BusNumber: 1, DeviceNumber: 19}}, nil
				},
				query: func(c Candidate) (CaptureResult, error) {
					queried = true
					q, err := queryCapture(first, true)
					q.result.PhysicalPath = c.PhysicalPath
					return q.result, err
				},
				apply: func(_ Candidate, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration) (bool, error)) (ApplyResult, error) {
					applied = true
					return applySettingsConfirmed(fresh, target, changes, true, time.Second, guard, true)
				},
				probeCH57: func(_ string, _ string, _ ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
					t.Fatal("write command probed CH57")
					return ch57.ProbeResult{}, nil
				},
			}
			var out, diagnostic bytes.Buffer
			err := runPublic(tc.args, &cliRuntime{strings.NewReader("y\n"), &out, &diagnostic}, access)
			if (err != nil) != tc.wantErr || queried != tc.wantXFKEY || applied != tc.wantXFKEY {
				t.Fatalf("args=%v err=%v queried=%v applied=%v out=%q diag=%q", tc.args, err, queried, applied, out.String(), diagnostic.String())
			}
			if tc.wantDiag != "" && !strings.Contains(diagnostic.String(), tc.wantDiag) {
				t.Fatalf("args=%v missing %q in diagnostic %q", tc.args, tc.wantDiag, diagnostic.String())
			}
			if tc.wantOutSubstr != "" && !strings.Contains(out.String(), tc.wantOutSubstr) {
				t.Fatalf("args=%v missing %q in output %q", tc.args, tc.wantOutSubstr, out.String())
			}
		}
	})
	t.Run("physical-path-sorting", func(t *testing.T) {
		var queriedPaths []string
		transport := newSettingsTransport()
		access := publicAccess{
			interactive: func(io.Reader) bool { return true },
			discover: func() ([]Candidate, error) {
				return []Candidate{
					{PhysicalPath: "7-1.4.3.1", Compatible: true},
					{PhysicalPath: "1-1.10", Compatible: true},
					{PhysicalPath: "1-1.2", Compatible: true},
				}, nil
			},
			discoverCH57: func(string) ([]ch57.DiscoveredCandidate, error) {
				return []ch57.DiscoveredCandidate{
					{Candidate: ch57.Candidate{PhysicalPath: "2-1"}, BusNumber: 2, DeviceNumber: 3},
				}, nil
			},
			query: func(c Candidate) (CaptureResult, error) {
				queriedPaths = append(queriedPaths, c.PhysicalPath)
				q, err := queryCapture(transport, true)
				q.result.PhysicalPath = c.PhysicalPath
				return q.result, err
			},
			probeCH57: func(_ string, _ string, selected ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
				queriedPaths = append(queriedPaths, selected.PhysicalPath)
				return ch57.ProbeResult{Status: ch57.ProbeRawReport, Raw: []byte{0xaa}, Count: 1}, nil
			},
		}
		var out, diagnostic bytes.Buffer
		if err := runPublic(nil, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access); err != nil {
			t.Fatal(err)
		}
		wantOrder := []string{"1-1.2", "1-1.10", "2-1", "7-1.4.3.1"}
		if !reflect.DeepEqual(queriedPaths, wantOrder) {
			t.Fatalf("got order %v, want %v", queriedPaths, wantOrder)
		}
		for _, heading := range []string{
			"[1] XFKEY One Key Max (0112)\n  USB path   1-1.2\n",
			"[2] XFKEY One Key Max (0112)\n  USB path   1-1.10\n",
			"[3] CH57x Macropad (1189:8890)\n  USB path   2-1\n",
			"[4] XFKEY One Key Max (0112)\n  USB path   7-1.4.3.1\n",
		} {
			if !strings.Contains(out.String(), heading) {
				t.Fatalf("missing %q in %q", heading, out.String())
			}
		}
		out.Reset()
		diagnostic.Reset()
		err := runPublic([]string{"key", "f13"}, &cliRuntime{strings.NewReader("y\n"), &out, &diagnostic}, access)
		wantErr := "Error: multiple compatible devices connected; specify --device <N>:\n" +
			"  --device 1  XFKEY One Key Max (0112) (USB path 1-1.2)\n" +
			"  --device 2  XFKEY One Key Max (0112) (USB path 1-1.10)\n" +
			"  --device 3  CH57x Macropad (1189:8890) (USB path 2-1)\n" +
			"  --device 4  XFKEY One Key Max (0112) (USB path 7-1.4.3.1)\n" +
			"Run \"wonkey --help\" for commands.\n"
		if err == nil || diagnostic.String() != wantErr {
			t.Fatalf("err=%v diagnostic=%q want=%q", err, diagnostic.String(), wantErr)
		}
	})
	t.Run("rejects-writes-on-ch57", func(t *testing.T) {
		for _, args := range [][]string{{"key", "f13"}, {"mouse", "left"}, {"media", "play"}, {"multi", "a"}, {"rgb", "off"}} {
			access := ch57TestAccess(t)
			access.discoverCH57 = func(string) ([]ch57.DiscoveredCandidate, error) {
				return []ch57.DiscoveredCandidate{{Candidate: ch57.Candidate{PhysicalPath: "1-2"}, BusNumber: 1, DeviceNumber: 19}}, nil
			}
			access.probeCH57 = func(string, string, ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
				t.Fatal("write command probed CH57")
				return ch57.ProbeResult{}, nil
			}
			var diagnostic bytes.Buffer
			err := runPublic(args, &cliRuntime{strings.NewReader("y\n"), io.Discard, &diagnostic}, access)
			if err == nil || ExitCode(err) != 1 || !strings.Contains(diagnostic.String(), "live writes are not supported for 1189:8890") {
				t.Fatalf("args=%v err=%v diagnostic=%q", args, err, diagnostic.String())
			}
		}
	})
}

func TestPublicCH57ProbeFailures(t *testing.T) {
	for _, result := range []ch57.ProbeResult{
		{Status: ch57.ProbeRefused},
		{Status: ch57.ProbeDescriptorFailed, ReportDescriptor: []byte{6, 0, 0xfe}, ReportDescriptorCount: 3},
		{Status: ch57.ProbeTransferFailed, Count: 0, ReportDescriptorVerified: true},
		{Status: ch57.ProbeInvalidReport, Raw: []byte{3, 0xaa}, Count: 2, ReportDescriptorVerified: true},
		{Status: ch57.ProbeCloseFailed, Raw: []byte{3, 0xbb}, Count: 2, ReportDescriptorVerified: true},
	} {
		t.Run(string(result.Status), func(t *testing.T) {
			access := ch57TestAccess(t)
			access.discoverCH57 = func(string) ([]ch57.DiscoveredCandidate, error) {
				return []ch57.DiscoveredCandidate{{Candidate: ch57.Candidate{PhysicalPath: "1-2"}}}, nil
			}
			failure := errors.New("synthetic probe failure")
			probes := 0
			access.probeCH57 = func(string, string, ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
				probes++
				return result, failure
			}
			var out, diagnostic bytes.Buffer
			err := runPublic(nil, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
			if !errors.Is(err, failure) || ExitCode(err) != 1 || probes != 1 || !strings.Contains(diagnostic.String(), failure.Error()) {
				t.Fatal(err, probes, diagnostic.String())
			}
			for _, text := range []string{"[1] CH57x Macropad (1189:8890)", "1-2", "3-key / 1-knob up to 12-key / 3-knob", string(result.Status)} {
				if !strings.Contains(out.String(), text) {
					t.Fatal("missing failure output", text, out.String())
				}
			}
			if result.Raw != nil && (!strings.Contains(out.String(), "Input bytes 2") || !strings.Contains(out.String(), fmt.Sprintf("%x", result.Raw))) {
				t.Fatal("missing input diagnostic", out.String())
			}
		})
	}
}

func TestPublicCH57OutputFailure(t *testing.T) {
	for _, needle := range []string{"[1] CH57x Macropad (1189:8890)", "USB path", "Layout", "Status", "Settings", "Input hex"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", needle, short), func(t *testing.T) {
				access := ch57TestAccess(t)
				access.discoverCH57 = func(string) ([]ch57.DiscoveredCandidate, error) {
					return []ch57.DiscoveredCandidate{{Candidate: ch57.Candidate{PhysicalPath: "1-2"}}}, nil
				}
				probes := 0
				access.probeCH57 = func(string, string, ch57.DiscoveredCandidate) (ch57.ProbeResult, error) {
					probes++
					return ch57.ProbeResult{Status: ch57.ProbeRawReport, Raw: []byte{3}, Count: 1}, nil
				}
				input := strings.NewReader("1\n")
				out := &failingHumanOutput{needle: needle, short: short}
				err := runPublic(nil, &cliRuntime{input, out, io.Discard}, access)
				want := io.ErrClosedPipe
				if short {
					want = io.ErrShortWrite
				}
				wantProbes := 0
				if needle == "Status" || needle == "Settings" || needle == "Input hex" {
					wantProbes = 1
				}
				if !errors.Is(err, want) || probes != wantProbes || input.Len() != len("1\n") {
					t.Fatal(err, probes, input.Len())
				}
			})
		}
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
	if err := runPublic(nil, &cliRuntime{strings.NewReader(""), io.Discard, io.Discard}, access); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("query created root", err)
	}
}

func TestPublicUnknownConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
	}{
		{"type-ff", 0, 0xff},
		{"key-count", 3, 2},
		{"key", 4, 0xff},
		{"trigger", 1, 0xff},
		{"modifiers", 2, 0xff},
		{"rgb-mode", 124, 0xff},
	} {
		for _, args := range [][]string{nil, {"key", "f13"}, {"key", "enter"}, {"rgb", "off"}, {"rgb", "cycle-slow"}} {
			t.Run(tc.name+"/"+strings.Join(args, " "), func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "absent-state")
				t.Setenv("XDG_STATE_HOME", root)
				transport := newSettingsTransport()
				transport.current[tc.offset] = tc.value
				original := transport.current
				query := len(args) == 0
				access := publicAccess{
					interactive: func(io.Reader) bool { return !query },
					discover: func() ([]Candidate, error) {
						return []Candidate{{PhysicalPath: "synthetic", Compatible: true}}, nil
					},
					query: func(c Candidate) (CaptureResult, error) {
						q, err := queryCapture(transport, true)
						q.result.PhysicalPath = c.PhysicalPath
						return q.result, err
					},
					apply: func(Candidate, ApplyTarget, configurationChanges, func(configuration, configuration) (bool, error)) (ApplyResult, error) {
						t.Fatal("unsupported configuration reached apply")
						return ApplyResult{}, nil
					},
				}
				var out, diagnostic bytes.Buffer
				input := strings.NewReader("yes\n")
				err := runPublic(args, &cliRuntime{input, &out, &diagnostic}, access)
				if query {
					want := fmt.Sprintf("1️⃣ WonKey\n\n[1] XFKEY One Key Max (0112)\n"+
						"  USB path   synthetic\n"+
						"  Identifier be077ba2\n"+
						"  Version    1014\n"+
						"  Layout     Unknown (unsupported configuration)\n"+
						"  Raw type   0x%02x (byte 0)\n  Raw 0-15   %x\n"+
						"Fields are not decoded. Changes are blocked.\n", original[0], original[:16])
					if err != nil || diagnostic.Len() != 0 || out.String() != want {
						t.Fatalf("error=%v diagnostic=%q output=%q want=%q", err, diagnostic.String(), out.String(), want)
					}
				} else if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "unsupported current") || out.String() != "1️⃣ WonKey\n\n" {
					t.Fatalf("error=%v output=%q", err, out.String())
				}
				if input.Len() != len("yes\n") || transport.current != original {
					t.Fatal("read confirmation or changed settings")
				}
				checkNoUpload(t, transport)
				if len(transport.packets) != 4 {
					t.Fatal("unexpected query count", len(transport.packets))
				}
				for i, opcode := range []byte{1, 6, 7, 8} {
					want := vendorPacket{0xaf, opcode}
					if transport.packets[i] != want {
						t.Fatalf("unexpected query packet %x", transport.packets[i])
					}
				}
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatal("created backup storage", err)
				}
			})
		}
	}
}

func TestPublicSupportedQueryOutput(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent-state")
	t.Setenv("XDG_STATE_HOME", root)
	transport := newSettingsTransport()
	transport.current[2] = 3
	access := publicAccess{
		interactive: func(io.Reader) bool { return false },
		discover: func() ([]Candidate, error) {
			return []Candidate{{PhysicalPath: "synthetic", Compatible: true}}, nil
		},
		query: func(c Candidate) (CaptureResult, error) {
			q, err := queryCapture(transport, true)
			q.result.PhysicalPath = c.PhysicalPath
			return q.result, err
		},
	}
	var out, diagnostic bytes.Buffer
	err := runPublic(nil, &cliRuntime{strings.NewReader(""), &out, &diagnostic}, access)
	want := "1️⃣ WonKey\n\n[1] XFKEY One Key Max (0112)\n" +
		"  USB path   synthetic\n" +
		"  Identifier be077ba2\n" +
		"  Version    1014\n" +
		"  Key        ctrl+shift+enter\n" +
		"  On         press\n" +
		"  Mode       cycle-slow\n" +
		"  Colour     #FFFFFF\n"
	if err != nil || diagnostic.Len() != 0 || out.String() != want {
		t.Fatalf("error=%v diagnostic=%q output=%q want=%q", err, diagnostic.String(), out.String(), want)
	}
	checkNoUpload(t, transport)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("created backup storage", err)
	}
}

func TestPublicLiveBoundaryOffline(t *testing.T) {
	for _, name := range []string{"write", "reordered", "descriptor", "node", "inode", "path", "missing", "open-failed"} {
		t.Run(name, func(t *testing.T) {
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
				return fresh, &closeRecorder{}, nil
			}
			access := publicDeviceAccess()
			access.interactive = func(io.Reader) bool { return true }
			var out bytes.Buffer
			err := runPublic([]string{"key", "f13", "--device", "1"}, &cliRuntime{strings.NewReader("y\n"), &out, io.Discard}, access)
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
					t.Fatal("fresh query or verification missing", out.String())
				}
			} else {
				checkNoUpload(t, fresh.settingsTransport)
			}
		})
	}
}
