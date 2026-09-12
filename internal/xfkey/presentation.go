package xfkey

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

func resolveCaptureRoot(value string) (string, error) {
	if value == "" {
		value = os.Getenv("WONKEY_CAPTURE_ROOT")
	}
	if value == "" {
		value = os.Getenv("XFKEY_CAPTURE_ROOT")
	}
	if value == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		value = filepath.Join(home, ".local", "state", "xfkey-captures")
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("backup root must resolve to an absolute directory")
	}
	return filepath.Clean(value), nil
}

type targetToken struct {
	PhysicalPath string `json:"physical_path"`
	Model        string `json:"model"`
	Identifier   string `json:"identifier"`
	Version      string `json:"version_hex"`
}

func encodeTarget(target targetToken) (string, error) {
	b, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	return "wonkey-target-v1:" + base64.RawURLEncoding.EncodeToString(b), nil
}

func compactTarget(target targetToken) string {
	return strings.Join([]string{target.Model, target.PhysicalPath, target.Identifier, target.Version}, ":")
}

var compactPath = regexp.MustCompile(`^[1-9][0-9]*-[1-9][0-9]*(\.[1-9][0-9]*)*$`)

func decodeTarget(value string) (targetToken, error) {
	var target targetToken
	const prefix = "wonkey-target-v1:"
	if !strings.HasPrefix(value, prefix) {
		parts := strings.Split(value, ":")
		if len(parts) != 4 || parts[0] != "0112" || !compactPath.MatchString(parts[1]) {
			return target, fmt.Errorf("invalid target %q: use the complete target from show --target (0112:path:identifier:version), or use wonkey-dev set instead", value)
		}
		target = targetToken{parts[1], parts[0], parts[2], parts[3]}
		if err := (ApplyTarget{target.Identifier, target.Version}).validate(); err != nil {
			return targetToken{}, fmt.Errorf("invalid target %q: identifier needs 8 hex digits and version needs 4; copy the complete target from show --target", value)
		}
		return target, nil
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return target, fmt.Errorf("invalid target %q: malformed encoding; copy the complete target from show --target", value)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil {
		return target, fmt.Errorf("invalid target %q: %w; copy the complete target from show --target", value, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF || target.PhysicalPath == "" || target.Model != "0112" {
		return targetToken{}, fmt.Errorf("invalid target %q: malformed or unsupported target; copy the complete target from show --target", value)
	}
	if err := (ApplyTarget{target.Identifier, target.Version}).validate(); err != nil {
		return targetToken{}, fmt.Errorf("invalid target %q: identifier needs 8 hex digits and version needs 4; copy the complete target from show --target", value)
	}
	return target, nil
}

type envelope struct {
	SchemaVersion       int          `json:"schema_version"`
	Command             string       `json:"command"`
	Outcome             string       `json:"outcome"`
	Device              *targetToken `json:"device,omitempty"`
	Target              string       `json:"target,omitempty"`
	Capture             string       `json:"capture_directory,omitempty"`
	Changes             []changeView `json:"changes"`
	WriteAttempted      bool         `json:"write_attempted"`
	ReadbackVerified    bool         `json:"readback_verified"`
	PersistenceVerified bool         `json:"persistence_after_reconnect_verified"`
	Warnings            []string     `json:"warnings"`
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
	mod := func(v byte) string {
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
	add("modifiers", mod(current[2]), mod(intended[2]))
	light := []string{"", "gradient", "steady", "flowing", "flash", "neon", "off", "held", "toggle"}
	add("lighting", light[current[124]], light[intended[124]])
	add("colour", fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127]), fmt.Sprintf("#%02X%02X%02X", intended[125], intended[126], intended[127]))
	return views
}
