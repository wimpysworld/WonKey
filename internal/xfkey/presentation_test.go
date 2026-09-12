package xfkey

import (
	"strings"
	"testing"
)

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

func TestSuperModifierPresentation(t *testing.T) {
	for _, tc := range []struct {
		mask byte
		want string
	}{
		{8, "super"},
		{12, "alt,super"},
		{15, "ctrl,shift,alt,super"},
	} {
		if got := modifierName(tc.mask); got != tc.want {
			t.Fatalf("modifierName(%d) = %q, want %q", tc.mask, got, tc.want)
		}
		changes, err := parseSettings("", "", tc.want, "", "")
		if err != nil || changes["modifiers"] != int(tc.mask) {
			t.Fatal(changes, err)
		}
	}
	current := syntheticSettings()
	intended := current
	intended[2] = 8
	views := settingViews(current, intended)
	if len(views) != 1 || views[0] != (changeView{"modifiers", "none", "super"}) {
		t.Fatal(views)
	}
}
