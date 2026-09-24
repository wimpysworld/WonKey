package xfkey

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func supportedKeyNames() map[byte]string {
	names := map[byte]string{0x27: "0", 0x28: "enter"}
	for name := byte('a'); name <= 'z'; name++ {
		names[0x04+name-'a'] = string(name)
	}
	for name := byte('1'); name <= '9'; name++ {
		names[0x1e+name-'1'] = string(name)
	}
	for number := 1; number <= 12; number++ {
		names[0x3a+byte(number-1)] = fmt.Sprintf("f%d", number)
		names[0x68+byte(number-1)] = fmt.Sprintf("f%d", number+12)
	}
	for start, group := range map[byte]string{
		0x29: "esc backspace tab space minus equal leftbracket rightbracket backslash nonushash semicolon apostrophe grave comma period slash capslock",
		0x46: "printscreen scrolllock pause insert home pageup delete end pagedown right left down up numlock keypaddivide keypadmultiply keypadminus keypadplus keypadenter keypad1 keypad2 keypad3 keypad4 keypad5 keypad6 keypad7 keypad8 keypad9 keypad0 keypadperiod nonusbackslash application power keypadequal",
		0x74: "execute help menu select stop again undo cut copy paste find mute volumeup volumedown lockingcapslock lockingnumlock lockingscrolllock keypadcomma keypadequalas400 international1 international2 international3 international4 international5 international6 international7 international8 international9 lang1 lang2",
	} {
		for offset, name := range strings.Fields(group) {
			names[start+byte(offset)] = name
		}
	}
	return names
}

func TestSupportedKeys(t *testing.T) {
	names := supportedKeyNames()
	if len(keyValues) != len(names) {
		t.Fatalf("got %d key names, want %d", len(keyValues), len(names))
	}
	for value, name := range names {
		usage := int(value)
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				expression string
				modifiers  byte
			}{
				{name, 0},
				{strings.ToUpper(name), 0},
				{"CTRL+SHIFT+ALT+SUPER+" + strings.ToUpper(name), 15},
			} {
				command, err := parsePublic([]string{"key", tc.expression})
				want := Changes{"key": usage, "modifiers": int(tc.modifiers)}
				if err != nil || !reflect.DeepEqual(command.changes, want) {
					t.Fatalf("parse %q: changes=%v error=%v", tc.expression, command.changes, err)
				}
				current := syntheticSettings()
				current[1], current[2] = 3, 7
				intended, err := changeConfiguration(current, command.changes)
				expected := current
				expected[2], expected[4] = tc.modifiers, value
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
	for _, name := range []string{
		"f0", "f25", "f01", "f014", "f24x", "!", ";", "+", "-", "0x04", "04", "145", "0x91", "0x92",
		"a+b", "ctrl+a+b", "ctrl+", "ctrl+ctrl+esc", "keypad10", "international0", "international10", "lang3",
		"none", "errorrollover", "postfail", "errorundefined", "reserved", "rightctrl+a", "altgr+a",
		"rightshift", "rightalt", "rightsuper", "leftctrl", "playpause", "return", " space", "space ",
	} {
		access := publicAccess{
			interactive: func(io.Reader) bool { t.Fatal("invalid key accessed runtime"); return false },
			discover:    func() ([]Candidate, error) { t.Fatal("invalid key accessed devices"); return nil, nil },
		}
		err := runPublic([]string{"key", name}, &cliRuntime{strings.NewReader(""), io.Discard, io.Discard}, access)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("unsupported key %q: %v", name, err)
		}
	}
	for usage := -1; usage <= 256; usage++ {
		want := usage >= 0x04 && usage <= 0x91
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

func TestKeyAliases(t *testing.T) {
	aliases := map[string]string{
		"escape": "esc", "pgup": "pageup", "pgdn": "pagedown",
		"arrowright": "right", "arrowleft": "left", "arrowdown": "down", "arrowup": "up",
	}
	if !reflect.DeepEqual(keyAliases, aliases) {
		t.Fatalf("unexpected aliases: %v", keyAliases)
	}
	for alias, canonical := range aliases {
		t.Run(alias, func(t *testing.T) {
			for _, name := range []string{alias, strings.ToUpper(alias)} {
				command, err := parsePublic([]string{"key", "ctrl+" + name})
				want := Changes{"key": keyValues[canonical], "modifiers": 1}
				if err != nil || !reflect.DeepEqual(command.changes, want) {
					t.Fatalf("alias %q: %v, %v", name, command.changes, err)
				}
				changes, err := parseSettings(name, "", "ctrl", "", "")
				if err != nil || !reflect.DeepEqual(changes, want) {
					t.Fatalf("settings alias %q: %v, %v", name, changes, err)
				}
				if got := settingName(keyValues, changes["key"]); got != canonical {
					t.Fatalf("alias display %q, want %q", got, canonical)
				}
			}
		})
	}
}

func TestSupportedKeyRestore(t *testing.T) {
	for usage, name := range supportedKeyNames() {
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
			changes, err := restoreChanges(source.config)
			if err != nil {
				t.Fatal(err)
			}
			result, err := applySettings(current, backup, settingsTarget(), changes, true, time.Second)
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
