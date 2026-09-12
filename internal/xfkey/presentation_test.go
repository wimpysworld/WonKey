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
