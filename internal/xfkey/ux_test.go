package xfkey

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func TestPositionalSettingsCompatibility(t *testing.T) {
	root, err := os.MkdirTemp(t.TempDir(), "capture with =")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := newCapture(root, Candidate{PhysicalPath: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := captureQueries(newSettingsTransport(), dir, true); err != nil {
		t.Fatal(err)
	}
	old, _, err := runCLI("plan", "--capture", dir, "--key", "f13", "--trigger", "release", "--modifiers", "ctrl,shift", "--lighting", "steady", "--colour", "0000ff", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"plan", dir, "key=f13", "trigger=release", "modifiers=ctrl,shift", "light=steady:0000ff", "--json"},
		{"plan", "--capture", dir, "key=f13", "trigger=release", "modifiers=ctrl,shift", "lighting=steady", "colour=#0000ff", "--json"},
		{"plan", dir, "--key", "f13", "--trigger", "release", "--modifiers", "ctrl,shift", "--lighting", "steady", "--colour", "0000ff", "--json"},
	} {
		got, stderr, err := runCLI(args...)
		if err != nil || stderr != "" || got != old {
			t.Fatalf("%v: %s %s %v", args, got, stderr, err)
		}
	}
	stdout, _, err := runCLI("plan", dir, "key=enter")
	if err != nil || !strings.Contains(stdout, "No changes needed in this backup") || !strings.Contains(stdout, "No device access") {
		t.Fatal(stdout, err)
	}
}

func TestUXParserRejectsConflictsBeforeAccess(t *testing.T) {
	for _, args := range [][]string{
		{"show", "1-1.2", "--device", "1-1.2"},
		{"show", "1-1.2", "--device="},
		{"show", "1-1.2", "1-1.3"},
		{"plan", "capture", "--capture", "other", "key=f13"},
		{"apply", "0112:1-1.2:be077ba2:1014", "--target", "other", "key=f13", "--write"},
		{"apply", "0112:1-1.2:be077ba2:1014", "--path", "1-1.2", "key=f13", "--write"},
	} {
		_, _, err := runCLI(args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, settings := range [][]string{
		{"key=f13", "key=enter"}, {"light=steady:0000ff", "colour=ff0000"},
		{"lighting=off", "light=steady:0000ff"}, {"colour=ffffff", "colour=000000"},
		{"key="}, {"unknown=value"}, {"light=steady"}, {"light=:ffffff"}, {"light=steady:"},
		{"light=steady:0000ff:extra"}, {"trigger=2"}, {"modifiers=3"},
		{"key=f13", "--lighting", "steady"}, {"key=f13", "--key="},
		{"key=f13", "--rgb-mode", "1"}, {"key=f13", "second-object"},
		{"--key", "f13", "--key", "enter"},
	} {
		args := append([]string{"plan", "unused-capture", "--json"}, settings...)
		stdout, stderr, err := runCLI(args...)
		if err == nil || ExitCode(err) != 2 || !strings.Contains(stderr, "plan --help") {
			t.Fatalf("%v: %v %s", args, err, stderr)
		}
		var value envelope
		if json.Unmarshal([]byte(stdout), &value) != nil {
			t.Fatal(stdout)
		}
	}
}

func TestPositionalShowAdapter(t *testing.T) {
	model := &cliModel{}
	parser, err := kong.New(model)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := parser.Parse([]string{"show", "1-1.2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := adaptCLI(model, ctx); err != nil || model.Show.Device != "1-1.2" {
		t.Fatal(model.Show, err)
	}
}

func TestPositionalApplyGateAndPreflight(t *testing.T) {
	for _, settings := range []string{"key=f13", "light=steady:0000ff"} {
		_, _, err := runCLI("apply", "0112:1-1.2:be077ba2:1014", settings)
		if !errors.Is(err, errWriteRequired) {
			t.Fatal(err)
		}
		_, _, err = runCLI("apply", "0112:1-1.2:be077ba2:1014", settings, "--write")
		if err == nil || !strings.Contains(err.Error(), "interactive input or --yes") {
			t.Fatal(err)
		}
	}
}

func TestApplyCompatibilityTargetsWithAssignments(t *testing.T) {
	token, err := encodeTarget(targetToken{"1-1.2", "0112", "be077ba2", "1014"})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range [][]string{
		{token}, {"--target", token},
		{"--path", "1-1.2", "--expect-identifier", "be077ba2", "--expect-version", "1014"},
	} {
		args := append(append([]string{"apply"}, target...), "key=f13", "--write")
		_, _, err := runCLI(args...)
		if err == nil || !strings.Contains(err.Error(), "interactive input or --yes") {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestCompactTargets(t *testing.T) {
	want := targetToken{"1-1.2", "0112", "be077ba2", "1014"}
	old, _ := encodeTarget(want)
	for _, text := range []string{compactTarget(want), old} {
		got, err := decodeTarget(text)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(got, err)
		}
	}
	for _, value := range []string{"0112:1-1.2:be077ba2", "0113:1-1.2:be077ba2:1014", "0112:/dev/hidraw0:be077ba2:1014", "0112:../1-1:be077ba2:1014", "0112:1-1.2:be077ba:1014", "0112:1-1.2:be077ba2:101", "0112:1-1.2:zz077ba2:1014", "0112:1-1.2:be077ba2:zzzz", "0112:1-1.2\n:be077ba2:1014", "0112:1-1.2:be077ba2:1014:extra", "0112:1-0:be077ba2:1014"} {
		if _, err := decodeTarget(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestHumanColourPolicyAndSwatch(t *testing.T) {
	for _, tc := range []struct {
		name, term, noColour, capability string
		tty, colour, rgb                 bool
	}{
		{"truecolour", "xterm-256color", "", "truecolor", true, true, true},
		{"24bit", "xterm", "", "24bit", true, true, true},
		{"ansi", "xterm", "", "", true, true, false},
		{"redirected", "xterm", "", "truecolor", false, false, false},
		{"no-colour", "xterm", "1", "truecolor", true, false, false},
		{"dumb", "dumb", "", "truecolor", true, false, false},
		{"unset-term", "", "", "truecolor", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			env := map[string]string{"TERM": tc.term, "NO_COLOR": tc.noColour, "COLORTERM": tc.capability}
			h := humanFor(&out, tc.tty, func(k string) string { return env[k] })
			h.heading("Current settings")
			out.WriteString(h.rgb("#1234EF"))
			got := out.String()
			if strings.Contains(got, "\x1b[") != tc.colour || strings.Contains(got, "\x1b[48;2;18;52;239m") != tc.rgb || !strings.Contains(got, "#1234EF") {
				t.Fatal(got)
			}
		})
	}
	t.Setenv("TERM", "xterm")
	t.Setenv("COLORTERM", "truecolor")
	var out bytes.Buffer
	if newHuman(&out).colour {
		t.Fatal("redirected stream has colour")
	}
}

func TestHumanEscapingAndCompleteTarget(t *testing.T) {
	var out bytes.Buffer
	h := newHuman(&out)
	target := targetToken{"1-1.2", "0112", "be077ba2", "1014"}
	h.show(target, syntheticSettings())
	if strings.Contains(out.String(), "Target") || strings.Contains(out.String(), compactTarget(target)) {
		t.Fatal("everyday output exposes target", out.String())
	}
	h.field("Target", compactTarget(target))
	if !strings.Contains(out.String(), compactTarget(target)) {
		t.Fatal("legacy target is not copyable", out.String())
	}
	out.Reset()
	attack := "path\x1b[31m\nforged\r\t\u202e"
	h.field("Records", attack)
	h.devices([]Candidate{{PhysicalPath: attack, Reasons: []string{attack}}})
	h.changes([]changeView{{attack, attack, attack}})
	got := out.String()
	if strings.ContainsAny(got, "\x1b\r\t\u202e") || strings.Contains(got, "\nforged") || !strings.Contains(got, `\x1b`) {
		t.Fatal(got)
	}
	stdout, stderr, err := runCLI("apply", "--target", attack, "--key", "f13", "--write", "--json")
	if err == nil || strings.ContainsRune(stderr, '\x1b') {
		t.Fatal(stderr, err)
	}
	var value envelope
	if json.Unmarshal([]byte(stdout), &value) != nil || strings.ContainsRune(stdout, '\x1b') {
		t.Fatal(stdout)
	}
}

func TestHumanSyntheticApplyOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, text       string
		change           int
		cancel, mismatch bool
		failAt           int
		failure          string
	}{
		{name: "verified", text: "All 128 bytes match intended settings", change: 0x68},
		{name: "no-op", text: "No changes needed. Backup saved. No settings write sent.", change: 0x28},
		{name: "cancelled", text: "Cancelled. Backup saved. No settings write sent.", change: 0x68, cancel: true},
		{name: "unknown", text: "Device state uncertain", change: 0x68, failAt: 5, failure: "write-error"},
		{name: "before-upload", text: "Apply stopped before settings upload", change: 0x68, failAt: 5, failure: "guard"},
		{name: "mismatch", text: "Readback differs from intended settings", change: 0x68, mismatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := newSettingsTransport()
			transport.failAt, transport.failure, transport.mismatch = tc.failAt, tc.failure, tc.mismatch
			dir := settingsDir(t)
			result, err := applySettingsConfirmed(transport, dir, settingsTarget(), Changes{"key": tc.change}, true, time.Second, func(configuration, configuration, string) (bool, error) { return !tc.cancel, nil })
			var out bytes.Buffer
			newHuman(&out).apply(result, err)
			got := out.String()
			if !strings.Contains(got, tc.text) || !strings.Contains(got, dir) || !strings.Contains(got, "Persistence after reconnect is not verified") {
				t.Fatal(got, err)
			}
			if err != nil && (!strings.Contains(got, "Do not retry automatically") || strings.Contains(got, "Settings written; readback verified")) {
				t.Fatal(got)
			}
			if tc.cancel || tc.change == 0x28 {
				checkNoUpload(t, transport)
			}
		})
	}
}

func TestHumanStorageFailureIsNotSuccess(t *testing.T) {
	dir := settingsDir(t)
	// Exclusive outcome creation must fail after the synthetic readback matches.
	if err := os.WriteFile(filepath.Join(dir, "apply-outcome.json"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := applySettings(newSettingsTransport(), dir, settingsTarget(), Changes{"key": 0x68}, true, time.Second)
	if err == nil || !result.ReadbackVerified {
		t.Fatal(result, err)
	}
	var out bytes.Buffer
	newHuman(&out).apply(result, err)
	if !strings.Contains(out.String(), "Record handling failed") || strings.Contains(out.String(), "Settings written; readback verified") {
		t.Fatal(out.String())
	}
}

func TestApplyFinishStreamsAndRetentionWarning(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			var stdout, stderr bytes.Buffer
			result := ApplyResult{Directory: "/backup", Outcome: "readback-verified", ReadbackVerified: true, WriteAttempted: true, Changes: []changeView{}, CleanupWarning: "retention failed"}
			var cause error
			if failed {
				result.Outcome, result.ReadbackVerified, result.CleanupWarning = "failed-state-unknown", false, ""
				cause = errors.New("submitted write failed")
			}
			command := applyCommand{outputOptions: outputOptions{JSON: jsonOutput}}
			err := command.finish(&cliRuntime{out: &stdout, errOut: &stderr}, targetToken{"1-1.2", "0112", "be077ba2", "1014"}, result, cause)
			if failed != (err != nil) {
				t.Fatal(err)
			}
			if err != nil {
				args := []string{"apply"}
				if jsonOutput {
					args = append(args, "--json")
				}
				reportCLIError(&cliModel{}, nil, args, &stdout, &stderr, err, 1)
			}
			if jsonOutput {
				var value envelope
				if json.Unmarshal(stdout.Bytes(), &value) != nil || value.Outcome != result.Outcome || value.WriteAttempted != result.WriteAttempted || value.ReadbackVerified != result.ReadbackVerified || value.PersistenceVerified {
					t.Fatal(stdout.String())
				}
			} else if failed {
				if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Device state uncertain") || !strings.Contains(stderr.String(), "/backup") {
					t.Fatal(stdout.String(), stderr.String())
				}
			} else if !strings.Contains(stdout.String(), "Persistence after reconnect is not verified") {
				t.Fatal(stdout.String())
			}
			if !failed && (!strings.Contains(stderr.String(), "retention failed") || !strings.Contains(stderr.String(), "Do not retry a successful apply")) {
				t.Fatal(stderr.String())
			}
		}
	}
}

func TestHumanNarrowOutputRetainsCopyableValues(t *testing.T) {
	var out bytes.Buffer
	h := newHuman(&out)
	h.width = 24
	h.line("", "Descriptions wrap at spaces on narrow terminals.")
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if len(line) > 24 {
			t.Fatal(out.String())
		}
	}
	out.Reset()
	token := "0112:123-123.123.123.123:be077ba2:1014"
	h.field("Target", token)
	if !strings.Contains(out.String(), "Target\n    "+token+"\n") {
		t.Fatal(out.String())
	}
}

func TestHumanEmptyDiscovery(t *testing.T) {
	var out bytes.Buffer
	newHuman(&out).devices(nil)
	if !strings.Contains(out.String(), "No matching devices found") || !strings.Contains(out.String(), "No HID device opened") {
		t.Fatal(out.String())
	}
}
