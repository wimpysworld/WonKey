package xfkey

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestExtendedFunctionKeys(t *testing.T) {
	for number := 14; number <= 24; number++ {
		name, usage := fmt.Sprintf("f%d", number), 0x69+number-14
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				expression string
				modifiers  int
			}{
				{name, 0},
				{"CTRL+SHIFT+ALT+SUPER+" + strings.ToUpper(name), 15},
			} {
				command, err := parsePublic([]string{"key", tc.expression})
				want := Changes{"key": usage, "modifiers": tc.modifiers}
				if err != nil || !reflect.DeepEqual(command.changes, want) {
					t.Fatalf("parse %q: changes=%v error=%v", tc.expression, command.changes, err)
				}
				current := syntheticSettings()
				current[1], current[2] = 3, 7
				intended, err := changeConfiguration(current, command.changes)
				expected := current
				expected[2], expected[4] = byte(tc.modifiers), byte(usage)
				if err != nil || intended != expected {
					t.Fatalf("key change lost omitted settings or unknown bytes: %v", err)
				}
				if err := supportedConfiguration(intended); err != nil {
					t.Fatal(err)
				}
				rgb, err := changeConfiguration(intended, Changes{"rgb-mode": 5})
				expected[124] = 6
				if err != nil || rgb != expected {
					t.Fatalf("RGB change lost extended key or unrelated bytes: %v", err)
				}
			}
			command, err := parsePublic([]string{"key", name, "--on", "release"})
			if err != nil || !reflect.DeepEqual(command.changes, Changes{"key": usage, "modifiers": 0, "trigger": 2}) {
				t.Fatal(command, err)
			}
		})
	}
}

func TestUnsupportedKeysRemainRejected(t *testing.T) {
	for _, name := range []string{"f0", "f1", "f12", "f25", "f014", "f24x", "escape", "a"} {
		if _, err := parsePublic([]string{"key", name}); err == nil {
			t.Fatalf("accepted unsupported key %q", name)
		}
	}
	for usage := -1; usage <= 256; usage++ {
		want := usage == 0x28 || (usage >= 0x68 && usage <= 0x73)
		if err := (Changes{"key": usage}).validate(); (err == nil) != want {
			t.Fatalf("usage %d: validation error=%v", usage, err)
		}
		if usage < 0 || usage > 255 {
			continue
		}
		current := syntheticSettings()
		current[4] = byte(usage)
		if err := supportedConfiguration(current); (err == nil) != want {
			t.Fatalf("usage %d: current layout error=%v", usage, err)
		}
		if _, err := changeConfiguration(current, Changes{"rgb-mode": 5}); (err == nil) != want {
			t.Fatalf("usage %d: RGB change error=%v", usage, err)
		}
	}
}

func TestExtendedFunctionKeyRestore(t *testing.T) {
	for number := 14; number <= 24; number++ {
		name, usage := fmt.Sprintf("f%d", number), byte(0x69+number-14)
		t.Run(name, func(t *testing.T) {
			root := publicTestCaptureRoot(t)
			saved := newSettingsTransport()
			saved.current = restoreTestSettings()
			saved.current[2], saved.current[4] = 15, usage
			directory := restoreTestSource(t, root, "20260912-123456-0000", saved)
			current := newSettingsTransport()
			current.current[63] ^= 1
			original := current.current
			identity, err := parseIdentity(current.identity)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			source, err := chooseRestore("", identity, original, bufio.NewReader(strings.NewReader("1\n")), newHuman(&out))
			if err != nil || source == nil || source.directory != directory || source.config != saved.current {
				t.Fatalf("restore selection=%v error=%v", source, err)
			}
			if !strings.Contains(out.String(), "ctrl+shift+alt+super+"+name+" (both)") {
				t.Fatalf("missing restore key name: %s", out.String())
			}
			backup := settingsDir(t)
			result, err := applySettings(current, backup, settingsTarget(), restoreChanges(source.config), true, time.Second)
			if err != nil || !result.ReadbackVerified || len(current.packets) != 11 {
				t.Fatalf("restore transaction=%v error=%v", result, err)
			}
			want := original
			for _, offset := range []int{1, 2, 4, 124, 125, 126, 127} {
				want[offset] = saved.current[offset]
			}
			if current.current != want {
				t.Fatal("restore lost saved settings or changed current unknown bytes")
			}
			_, captured, err := loadCapture(backup)
			if err != nil || captured != original {
				t.Fatalf("backup lost pre-restore state: %v", err)
			}
		})
	}
}
