package xfkey

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var settingOrder = []string{"key", "trigger", "modifiers", "rgb-mode", "red", "green", "blue"}

var keyValues = map[string]int{"enter": 0x28, "f13": 0x68}
var triggerValues = map[string]int{"press": 1, "release": 2, "both": 3}
var lightingValues = map[string]int{"gradient": 0, "steady": 1, "flowing": 2, "flash": 3, "neon": 4, "off": 5, "held": 6, "toggle": 7}

func parseSettings(key, trigger, modifiers, lighting, colour string) (Changes, error) {
	changes := Changes{}
	if key != "" {
		value, ok := keyValues[strings.ToLower(key)]
		if !ok {
			return nil, fmt.Errorf("invalid --key %q; accepted values: enter, f13", key)
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
				bit, ok := map[string]int{"ctrl": 1, "shift": 2, "alt": 4, "gui": 8}[name]
				if !ok || seen[name] {
					return nil, fmt.Errorf("invalid --modifiers %q; accepted values: none or comma-separated ctrl,shift,alt,gui", modifiers)
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
			return nil, fmt.Errorf("invalid --lighting %q; accepted values: gradient, steady, flowing, flash, neon, off, held, toggle", lighting)
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

func modifierName(v byte) string {
	if v == 0 {
		return "none"
	}
	names := []string{}
	for _, x := range []struct {
		name string
		bit  byte
	}{{"ctrl", 1}, {"shift", 2}, {"alt", 4}, {"gui", 8}} {
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
	add("key", map[byte]string{0x28: "enter", 0x68: "f13"}[current[4]], map[byte]string{0x28: "enter", 0x68: "f13"}[intended[4]])
	add("trigger", map[byte]string{1: "press", 2: "release", 3: "both"}[current[1]], map[byte]string{1: "press", 2: "release", 3: "both"}[intended[1]])
	add("modifiers", modifierName(current[2]), modifierName(intended[2]))
	light := []string{"", "gradient", "steady", "flowing", "flash", "neon", "off", "held", "toggle"}
	add("lighting", light[current[124]], light[intended[124]])
	add("colour", fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127]), fmt.Sprintf("#%02X%02X%02X", intended[125], intended[126], intended[127]))
	return views
}
