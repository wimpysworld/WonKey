package xfkey

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runDeveloperCLI(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := RunDeveloperIO(args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestLandingPageIsTheStandardHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}} {
		stdout, stderr, err := runDeveloperCLI(args...)
		if err != nil || stdout != landingPage || stderr != "" {
			t.Fatalf("args=%v stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
	for _, text := range []string{"WonKey  One key. Your rules.", "Commands:", "wonkey-dev show", "wonkey-dev set key=f13", "Advanced examples:", "wonkey-dev advanced", "--dry-run", "--device PHYSICAL_PATH", `For command help, run "wonkey-dev <command> --help".`, "--json"} {
		if !strings.Contains(landingPage, text) {
			t.Fatalf("landing page omits %q", text)
		}
	}
	for _, hidden := range []string{"wonkey-dev inspect", "wonkey-dev identify", "wonkey-dev readback", "wonkey-dev preview", "wonkey-dev apply", "wonkey-dev plan", "wonkey-dev devices", "wonkey-dev protocol"} {
		if strings.Contains(landingPage, hidden) {
			t.Fatalf("landing page exposes compatibility command %q", hidden)
		}
	}
}

func TestCommandHelpExplainsAccessAndExamples(t *testing.T) {
	checks := map[string][]string{
		"devices":  {"metadata", "does not open a HID device", "Example: wonkey-dev advanced devices"},
		"show":     {"Reads current settings", "creates no files", "only compatible device", "Example: wonkey-dev show"},
		"set":      {"saves and checks a new backup", "--dry-run reads the device", "wonkey-dev set key=f13", "exactly write"},
		"plan":     {"local files", "does not access hardware", "Example: wonkey-dev advanced plan"},
		"apply":    {"writes only explicit changes", "requires --write", "Example: wonkey-dev apply"},
		"protocol": {"always write one JSON value", "do not access hardware"},
	}
	for command, want := range checks {
		stdout, stderr, err := runDeveloperCLI("help", command)
		if err != nil || stderr != "" {
			t.Fatalf("%s help stderr=%q error=%v", command, stderr, err)
		}
		for _, text := range want {
			if !strings.Contains(strings.Join(strings.Fields(stdout), " "), text) {
				t.Fatalf("%s help omits %q:\n%s", command, text, stdout)
			}
		}
	}
	stdout, _, err := runDeveloperCLI("help", "protocol", "preview")
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--modifiers", "--trigger", "--rgb-mode", "--red", "--green", "--blue"} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("protocol preview help omits %s:\n%s", flag, stdout)
		}
	}
	for _, command := range []string{"inspect", "identify", "readback", "preview", "parse-identify", "parse-readback"} {
		stdout, _, err = runDeveloperCLI(command, "--help")
		if err != nil || stdout == "" {
			t.Fatalf("%s compatibility help = %q, %v", command, stdout, err)
		}
	}
}

func TestSyntaxErrorsAreActionable(t *testing.T) {
	for _, tc := range []struct {
		args []string
		hint string
	}{
		{[]string{"plan"}, `Run "wonkey-dev plan --help" for an offline example.`},
		{[]string{"plan", "--capture", ".", "--unknown"}, `Run "wonkey-dev plan --help" for an offline example.`},
		{[]string{"unknown"}, `Run "wonkey-dev --help" for available commands.`},
	} {
		stdout, stderr, err := runDeveloperCLI(tc.args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("args=%v error=%v code=%d", tc.args, err, ExitCode(err))
		}
		if stdout != "" || !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, tc.hint) {
			t.Fatalf("args=%v stdout=%q stderr=%q", tc.args, stdout, stderr)
		}
	}
}

func TestApplyPreflightNeverUsesDevice(t *testing.T) {
	_, _, err := runDeveloperCLI("apply", "--lighting", "steady", "--colour", "0000ff")
	if !errors.Is(err, errWriteRequired) {
		t.Fatalf("documented wrapper settings failed parsing: %v", err)
	}
	_, _, err = runDeveloperCLI("apply", "--rgb-mode", "1")
	if !errors.Is(err, errWriteRequired) {
		t.Fatalf("legacy setting flag failed parsing: %v", err)
	}
	_, _, err = runDeveloperCLI("apply", "--target", "bad", "--key", "f13")
	if err == nil || err.Error() != errWriteRequired.Error() {
		t.Fatalf("error = %v", err)
	}
	token, err := encodeTarget(targetToken{"1-1.2", "0112", "be077ba2", "1014"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = runDeveloperCLI("apply", "--target", token, "--write", "--key", "f13")
	if err == nil || !strings.Contains(err.Error(), "interactive input or --yes") {
		t.Fatalf("error = %v", err)
	}
}

func TestJSONConfirmationPromptUsesStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	writeConfirmationPrompt(&cliRuntime{out: &stdout, errOut: &stderr}, true)
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "Type \"write\" to change stored settings [cancel]: "; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestApplyRejectsDevNullAndInexactConfirmation(t *testing.T) {
	stdin, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	token, err := encodeTarget(targetToken{"1-1.2", "0112", "be077ba2", "1014"})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = RunDeveloperIO([]string{"apply", "--target", token, "--write", "--key", "f13"}, stdin, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "interactive input or --yes") {
		t.Fatalf("error = %v", err)
	}
	for _, input := range []string{"", "\n", "\r\n", "WRITE", "yes", " write ", "write\t", "write \n", " write\n"} {
		if exactWriteConfirmation(input) {
			t.Fatalf("accepted %q", input)
		}
	}
	for _, input := range []string{"write", "write\n", "write\r\n"} {
		if !exactWriteConfirmation(input) {
			t.Fatalf("rejected %q", input)
		}
	}
}

func TestJSONFailuresUseEnvelopeAndStderr(t *testing.T) {
	for _, args := range [][]string{
		{"plan", "--json"},
		{"plan", "--capture", "/nope", "--key", "f13", "--json"},
		{"apply", "--target", "bad", "--write", "--yes", "--key", "f13", "--json"},
		{"apply", "--json"},
		{"apply", "--json", "--bogus"},
		{"protocol", "parse-identify", "--hex", "af01"},
		{"protocol", "preview", "--replace-all"},
	} {
		stdout, stderr, err := runDeveloperCLI(args...)
		if err == nil {
			t.Fatalf("accepted %v", args)
		}
		var result envelope
		if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
			t.Fatalf("%v output is not one JSON value: %q: %v", args, stdout, decodeErr)
		}
		if result.SchemaVersion != 1 || result.Outcome == "" || len(result.Warnings) != 1 {
			t.Fatalf("%v envelope = %#v", args, result)
		}
		if !strings.HasPrefix(stderr, "Error: ") {
			t.Fatalf("%v stderr = %q", args, stderr)
		}
		if args[0] == "protocol" && !strings.Contains(stderr, `Run "wonkey-dev protocol `+args[1]+` --help"`) {
			t.Fatalf("%v lacks contextual help: %q", args, stderr)
		}
	}
}

func TestLegacyNumericSettingsRemainCompatibleAndHidden(t *testing.T) {
	changes, err := settingsChanges(settingsOptions{Key: "f13", Trigger: "2", Modifiers: "3", LegacyRGBMode: 1, LegacyRed: 255, LegacyGreen: 0, LegacyBlue: 4})
	if err != nil {
		t.Fatal(err)
	}
	want := Changes{"key": 0x68, "trigger": 2, "modifiers": 3, "rgb-mode": 1, "red": 255, "green": 0, "blue": 4}
	for name, value := range want {
		if changes[name] != value {
			t.Fatalf("%s = %d, want %d", name, changes[name], value)
		}
	}
	dir := settingsDir(t)
	if _, err = captureQueries(newSettingsTransport(), dir, true); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runDeveloperCLI("plan", "--capture", dir, "--trigger", "2", "--modifiers", "3", "--rgb-mode", "1", "--red", "255", "--green", "0", "--blue", "4")
	if err != nil || stderr != "" || !strings.Contains(stdout, "trigger: press -> release") || !strings.Contains(stdout, "colour: #FFFFFF -> #FF0004") {
		t.Fatalf("legacy plan stdout=%q stderr=%q error=%v", stdout, stderr, err)
	}
	for _, command := range []string{"plan", "apply"} {
		stdout, _, helpErr := runDeveloperCLI(command, "--help")
		if helpErr != nil {
			t.Fatal(helpErr)
		}
		for _, hidden := range []string{"--rgb-mode", "--red", "--green", "--blue"} {
			if strings.Contains(stdout, hidden) {
				t.Fatalf("%s help exposes %s:\n%s", command, hidden, stdout)
			}
		}
	}
	if _, _, err = runDeveloperCLI("apply", "--trigger", "2", "--modifiers", "3", "--rgb-mode", "1"); !errors.Is(err, errWriteRequired) {
		t.Fatalf("legacy apply flags failed parsing: %v", err)
	}
	for _, options := range []settingsOptions{
		{Trigger: "press", LegacyRGBMode: 1},
		{Modifiers: "ctrl", LegacyRed: 1},
		{Lighting: "steady", LegacyBlue: 1},
		{Colour: "0000ff", LegacyGreen: 1},
	} {
		if _, err = settingsChanges(options); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("accepted mixed settings: %#v, %v", options, err)
		}
	}
}

func TestPreviewWarnsAboutCompleteReplacement(t *testing.T) {
	stdout, _, err := runDeveloperCLI("protocol", "preview", "--replace-all", "--key", "f13", "--modifiers", "3", "--trigger", "2", "--rgb-mode", "1", "--red", "255", "--green", "0", "--blue", "4")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Complete replacement: all unspecified fields reset to zero") || !strings.Contains(stdout, "vendor_payloads_64_bytes_hex") {
		t.Fatal(stdout)
	}
}

func TestShowHelpHasNoCaptureRoot(t *testing.T) {
	stdout, stderr, err := runDeveloperCLI("show", "--help")
	if err != nil || stderr != "" || strings.Contains(stdout, "capture-root") || strings.Contains(stdout, "capture directory") {
		t.Fatalf("stdout=%q stderr=%q error=%v", stdout, stderr, err)
	}
}

func TestSettingsValidationOrderAndFriendlyValues(t *testing.T) {
	changes, err := parseSettings("f13", "release", "ctrl,shift", "steady", "#0000ff")
	if err != nil {
		t.Fatal(err)
	}
	want := Changes{"key": 0x68, "trigger": 2, "modifiers": 3, "rgb-mode": 1, "red": 0, "green": 0, "blue": 255}
	for key, value := range want {
		if changes[key] != value {
			t.Fatalf("%s = %d", key, changes[key])
		}
	}
	for i := 0; i < 20; i++ {
		err = (Changes{"blue": 999, "key": 1}).validate()
		if err == nil || !strings.Contains(err.Error(), "key=1") {
			t.Fatalf("non-deterministic error: %v", err)
		}
	}
}

func TestTargetTokenStrict(t *testing.T) {
	token, err := encodeTarget(targetToken{"1-1.2", "0112", "be077ba2", "1014"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := decodeTarget(token)
	if err != nil || target.PhysicalPath != "1-1.2" {
		t.Fatalf("target = %#v, %v", target, err)
	}
	for _, value := range []string{"wonkey-target-v2:x", token + "x", "wonkey-target-v1:eyJtb2RlbCI6IjAxMTIiLCJ1bmtub3duIjp0cnVlfQ"} {
		if _, err := decodeTarget(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestCaptureRootPrecedence(t *testing.T) {
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("XFKEY_CAPTURE_ROOT", "/xfkey")
	t.Setenv("WONKEY_CAPTURE_ROOT", "/wonkey")
	for input, want := range map[string]string{"/flag": "/flag", "": "/wonkey"} {
		got, err := resolveCaptureRoot(input)
		if err != nil || got != want {
			t.Fatalf("root = %q, %v", got, err)
		}
	}
	os.Unsetenv("WONKEY_CAPTURE_ROOT")
	if got, _ := resolveCaptureRoot(""); got != "/xfkey" {
		t.Fatal(got)
	}
	os.Unsetenv("XFKEY_CAPTURE_ROOT")
	if got, _ := resolveCaptureRoot(""); !strings.HasSuffix(got, "/.local/state/xfkey-captures") {
		t.Fatal(got)
	}
	if _, err := resolveCaptureRoot("relative"); err == nil {
		t.Fatal("accepted relative root")
	}
}
