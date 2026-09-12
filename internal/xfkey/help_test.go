package xfkey

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/kong"
)

func TestHelpReferenceAndLayout(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"set", "--help"}, {"set", "-h"}, {"help", "set"}, {"set", "key=f13", "--json", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := runDeveloperCLI(args...)
			if err != nil || stderr != "" {
				t.Fatal(stderr, err)
			}
			want := landingPage
			if len(args) > 1 && (args[0] == "set" || args[1] == "set") {
				want = setPage
			}
			if stdout != want {
				t.Fatalf("help differs from reference:\n%s", stdout)
			}
			for _, line := range strings.Split(stdout, "\n") {
				if utf8.RuneCountInString(line) > 90 || strings.ContainsAny(line, "\t\x1b") {
					t.Fatalf("unaligned or over-width line: %q", line)
				}
			}
			for _, section := range []string{"Settings (setting=value):", "Lighting modes:", "Saving settings:", "Backup location (first non-empty value wins):"} {
				if strings.Count(stdout, section) != 1 {
					t.Fatalf("missing or repeated section %q", section)
				}
			}
			for _, form := range []string{"key=KEY", "trigger=WHEN", "modifiers=LIST", "lighting=MODE", "colour=RGB", "light=MODE:RGB"} {
				assertHelpRow(t, stdout, form, "")
			}
			for mode, index := range lightingValues {
				assertHelpRow(t, stdout, mode, rgbLabels[index])
			}
			for key := range keyValues {
				if !strings.Contains(stdout, key) {
					t.Fatalf("missing key %q", key)
				}
			}
			for trigger := range triggerValues {
				if !strings.Contains(stdout, trigger) {
					t.Fatalf("missing trigger %q", trigger)
				}
			}
			for _, text := range []string{
				"No other keys are supported", "none, or comma-separated ctrl,shift,alt,gui without repeats",
				"six hexadecimal RGB digits", "#RRGGBB", "0000ff is blue", "Unspecified settings stay unchanged",
				"Do not combine light= with lighting= or colour=", "Most effects are not verified on hardware",
				"gui is the Super/Windows/Command modifier", "There are no brightness or speed options",
				"Type exactly write to confirm. A blank answer cancels", "set saves and checks a new backup",
				"If the device identity or settings change after confirmation, set stops",
				"If nothing changes or you cancel, set writes no settings",
				"--dry-run reads the device, so it is not offline", "Save no files and write no settings",
				"Skip confirmation, not safety checks or backup", "Absolute directory for new backups",
				"Selects the only compatible device", "Required when multiple devices match",
				"Write one stable JSON value to stdout", "Diagnostics and confirmation prompts stay on stderr",
				"--capture-root, then WONKEY_CAPTURE_ROOT, then XFKEY_CAPTURE_ROOT,",
				"then $HOME/.local/state/xfkey-captures.",
			} {
				if !strings.Contains(stdout, text) {
					t.Fatalf("missing reference %q", text)
				}
			}
			for _, hidden := range []string{"Settings flags", "Flags:", "Arguments:", "--rgb-mode", "--key", "AF02", "--write", "--target"} {
				if strings.Contains(stdout, hidden) {
					t.Fatalf("generated or legacy help leaked: %q", hidden)
				}
			}
		})
	}
}

func assertHelpRow(t *testing.T, text, label, description string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "  "+label+" ") {
			continue
		}
		got := strings.TrimSpace(strings.TrimPrefix(line, "  "+label))
		if strings.Index(line, got) != 20 || (description != "" && got != description) {
			t.Fatalf("incorrect reference row: %q", line)
		}
		return
	}
	t.Fatalf("missing reference row %q", label)
}

func TestHelpPublicOptionScopes(t *testing.T) {
	for _, text := range []string{
		"show, set. Select a physical USB path", "--dry-run             set only.",
		"--yes                 set only.", "--capture-root DIR    set only.",
		"--json                show, set, advanced devices, advanced plan.",
		"Advanced protocol tools always produce JSON.", "All commands. Show help without device access.",
	} {
		if !strings.Contains(landingPage, text) {
			t.Fatalf("missing option scope %q", text)
		}
	}
	for _, command := range []string{"show", "set", "advanced"} {
		assertHelpRow(t, landingPage, command, "")
	}
}

func TestHelpExamplesParseWithoutRunningCommands(t *testing.T) {
	for _, page := range []string{landingPage, setPage} {
		for _, line := range strings.Split(page, "\n") {
			if !strings.HasPrefix(line, "  wonkey-dev ") {
				continue
			}
			args := strings.Fields(line)[1:]
			model := &developerModel{}
			parser, err := kong.New(model)
			if err != nil {
				t.Fatal(err)
			}
			ctx, err := parser.Parse(args)
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}
			if err := adaptCLI(model, ctx); err != nil {
				t.Fatalf("%s: %v", line, err)
			}
			if args[0] == "set" {
				changes, err := settingsChanges(model.Set.settingsOptions)
				if err != nil || len(changes) == 0 {
					t.Fatalf("%s: %v, %v", line, changes, err)
				}
			}
		}
	}
}

func TestHelpSettingsUseCanonicalGrammar(t *testing.T) {
	for mode, value := range lightingValues {
		separate := parseSet(t, "lighting="+mode, "colour=#0000ff")
		compound := parseSet(t, "light="+mode+":0000ff")
		a, errA := settingsChanges(separate.settingsOptions)
		b, errB := settingsChanges(compound.settingsOptions)
		want := Changes{"rgb-mode": value, "red": 0, "green": 0, "blue": 255}
		if errA != nil || errB != nil || !reflect.DeepEqual(a, want) || !reflect.DeepEqual(b, want) {
			t.Fatal(mode, a, b, errA, errB)
		}
	}
	for key, value := range keyValues {
		command := parseSet(t, "key="+key)
		got, err := settingsChanges(command.settingsOptions)
		if err != nil || !reflect.DeepEqual(got, Changes{"key": value}) {
			t.Fatal(key, got, err)
		}
	}
	for trigger, value := range triggerValues {
		command := parseSet(t, "trigger="+trigger, "modifiers=none")
		got, err := settingsChanges(command.settingsOptions)
		if err != nil || !reflect.DeepEqual(got, Changes{"trigger": value, "modifiers": 0}) {
			t.Fatal(trigger, got, err)
		}
	}
	command := parseSet(t, "modifiers=ctrl,shift,alt,gui")
	got, err := settingsChanges(command.settingsOptions)
	if err != nil || !reflect.DeepEqual(got, Changes{"modifiers": 15}) {
		t.Fatal(got, err)
	}
}
