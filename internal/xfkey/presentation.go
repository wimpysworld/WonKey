package xfkey

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var settingOrder = []string{"key", "trigger", "modifiers", "rgb-mode", "red", "green", "blue"}

var (
	keyValues = map[string]int{
		"a": 0x04, "b": 0x05, "c": 0x06, "d": 0x07, "e": 0x08, "f": 0x09,
		"g": 0x0a, "h": 0x0b, "i": 0x0c, "j": 0x0d, "k": 0x0e, "l": 0x0f,
		"m": 0x10, "n": 0x11, "o": 0x12, "p": 0x13, "q": 0x14, "r": 0x15,
		"s": 0x16, "t": 0x17, "u": 0x18, "v": 0x19, "w": 0x1a, "x": 0x1b,
		"y": 0x1c, "z": 0x1d,
		"1": 0x1e, "2": 0x1f, "3": 0x20, "4": 0x21, "5": 0x22,
		"6": 0x23, "7": 0x24, "8": 0x25, "9": 0x26, "0": 0x27,
		"enter": 0x28,
		"f1":    0x3a, "f2": 0x3b, "f3": 0x3c, "f4": 0x3d,
		"f5": 0x3e, "f6": 0x3f, "f7": 0x40, "f8": 0x41,
		"f9": 0x42, "f10": 0x43, "f11": 0x44, "f12": 0x45,
		"f13": 0x68, "f14": 0x69, "f15": 0x6a, "f16": 0x6b,
		"f17": 0x6c, "f18": 0x6d, "f19": 0x6e, "f20": 0x6f,
		"f21": 0x70, "f22": 0x71, "f23": 0x72, "f24": 0x73,
	}
	triggerValues  = map[string]int{"press": 1, "release": 2, "both": 3}
	lightingValues = map[string]int{"static": 1, "breathe": 2, "cycle-slow": 0, "cycle-fast": 4, "flash": 3, "held": 6, "toggle": 7, "off": 5}
)

func parseSettings(key, trigger, modifiers, lighting, colour string) (Changes, error) {
	changes := Changes{}
	if key != "" {
		value, ok := keyValues[strings.ToLower(key)]
		if !ok {
			return nil, fmt.Errorf("invalid --key %q; accepted values: a-z, 0-9, enter, f1 through f24", key)
		}
		changes["key"] = value
	}
	if trigger != "" {
		value, ok := triggerValues[strings.ToLower(trigger)]
		if !ok {
			return nil, fmt.Errorf("invalid --trigger %q; accepted values: press, release, both", trigger)
		}
		changes["trigger"] = value
	}
	if modifiers != "" {
		mask := 0
		seen := map[string]bool{}
		if strings.ToLower(modifiers) != "none" {
			for _, name := range strings.Split(strings.ToLower(modifiers), ",") {
				bit, ok := map[string]int{"ctrl": 1, "shift": 2, "alt": 4, "super": 8}[name]
				if !ok || seen[name] {
					return nil, fmt.Errorf("invalid --modifiers %q; accepted values: none or comma-separated ctrl,shift,alt,super", modifiers)
				}
				seen[name] = true
				mask |= bit
			}
		}
		changes["modifiers"] = mask
	}
	if lighting != "" {
		value, ok := lightingValues[strings.ToLower(lighting)]
		if !ok {
			return nil, fmt.Errorf("invalid --lighting %q; accepted values: static, breathe, cycle-slow, cycle-fast, flash, held, toggle, off", lighting)
		}
		changes["rgb-mode"] = value
	}
	if colour != "" {
		value := strings.TrimPrefix(colour, "#")
		if len(value) != 6 {
			return nil, fmt.Errorf("invalid --colour %q; accepted value: RRGGBB or #RRGGBB", colour)
		}
		rgb, err := hex.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid --colour %q; accepted value: RRGGBB or #RRGGBB", colour)
		}
		changes["red"], changes["green"], changes["blue"] = int(rgb[0]), int(rgb[1]), int(rgb[2])
	}
	if err := changes.validate(); err != nil {
		return nil, err
	}
	return changes, nil
}

func resolveCaptureRoot() (string, error) {
	if state := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(state) {
		return filepath.Join(state, "wonkey", "captures"), nil
	}
	home := os.Getenv("HOME")
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("backup storage requires an absolute HOME when XDG_STATE_HOME is not absolute")
	}
	return filepath.Join(home, ".local", "state", "wonkey", "captures"), nil
}

func settingName(values map[string]int, value int) string {
	for name, candidate := range values {
		if candidate == value {
			return name
		}
	}
	return ""
}

func modifierName(v byte) string {
	if v == 0 {
		return "none"
	}
	names := []string{}
	for _, x := range []struct {
		name string
		bit  byte
	}{{"ctrl", 1}, {"shift", 2}, {"alt", 4}, {"super", 8}} {
		if v&x.bit != 0 {
			names = append(names, x.name)
		}
	}
	return strings.Join(names, ",")
}

type changeView struct {
	Setting string `json:"setting"`
	Before  string `json:"before"`
	After   string `json:"after"`
}

func settingViews(current, intended configuration) []changeView {
	views := []changeView{}
	add := func(name, before, after string) {
		if before != after {
			views = append(views, changeView{name, before, after})
		}
	}
	add("key", settingName(keyValues, int(current[4])), settingName(keyValues, int(intended[4])))
	add("trigger", settingName(triggerValues, int(current[1])), settingName(triggerValues, int(intended[1])))
	add("modifiers", modifierName(current[2]), modifierName(intended[2]))
	add("lighting", settingName(lightingValues, int(current[124])-1), settingName(lightingValues, int(intended[124])-1))
	add("colour", fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127]), fmt.Sprintf("#%02X%02X%02X", intended[125], intended[126], intended[127]))
	return views
}
