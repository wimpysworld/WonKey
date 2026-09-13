package xfkey

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPublicRGBNamesRoundTrip(t *testing.T) {
	modes := []struct {
		name, description string
		stored            byte
		colour            bool
	}{
		{"static", "Single static colour", 2, true},
		{"breathe", "Single breathing colour", 3, true},
		{"cycle-slow", "Full-colour slow cycle", 1, false},
		{"cycle-fast", "Full-colour fast cycle", 5, false},
		{"flash", "Flash on click", 4, true},
		{"held", "On while pressed, off on release", 7, true},
		{"toggle", "Toggle on click", 8, true},
		{"off", "Lights off", 6, false},
	}
	if len(lightingValues) != len(modes) {
		t.Fatal("unexpected public modes", lightingValues)
	}
	var helpRows strings.Builder
	for _, mode := range modes {
		label := mode.name
		if mode.colour {
			label += " [RGB]"
		}
		fmt.Fprintf(&helpRows, "  %-15s%s\n", label, mode.description)
		t.Run(mode.name, func(t *testing.T) {
			command, err := parsePublic([]string{"rgb", mode.name})
			if err != nil || command.changes["rgb-mode"] != int(mode.stored)-1 {
				t.Fatal(command, err)
			}
			current := syntheticSettings()
			changed, err := changeConfiguration(current, command.changes)
			if err != nil || changed[124] != mode.stored || settingName(lightingValues, int(changed[124])-1) != mode.name {
				t.Fatal(changed, err)
			}
			for offset := range current {
				if offset != 124 && changed[offset] != current[offset] {
					t.Fatal("changed unrelated byte", offset)
				}
			}
			var output bytes.Buffer
			command.showCurrentSettings(changed, Candidate{}, ApplyTarget{}, newHuman(&output))
			if !strings.Contains(output.String(), fmt.Sprintf("  %-10s %s\n", "Mode", mode.name)) {
				t.Fatal(output.String())
			}
			root := publicTestCaptureRoot(t)
			saved := newSettingsTransport()
			saved.current = changed
			source := restoreTestSource(t, root, "20260912-123456-0000", saved)
			identity, err := parseIdentity(saved.identity)
			if err != nil {
				t.Fatal(err)
			}
			current[124] = mode.stored%8 + 1
			output.Reset()
			selected, err := chooseRestore("", identity, current, bufio.NewReader(strings.NewReader("1\n")), newHuman(&output))
			if err != nil || selected == nil || selected.directory != source || !strings.Contains(output.String(), " | "+mode.name+" #") {
				t.Fatal(selected, err, output.String())
			}
			restored, err := changeConfiguration(current, restoreChanges(selected.config))
			if err != nil || restored != changed {
				t.Fatal("restore changed saved settings", err)
			}
		})
	}
	if !strings.Contains(rgbHelp, "Modes:\n"+helpRows.String()+"\nOptions:") {
		t.Fatal("mode order or descriptions differ", rgbHelp)
	}
}
