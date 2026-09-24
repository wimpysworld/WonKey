package xfkey

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTypedCaptureLabels(t *testing.T) {
	for _, tc := range []struct {
		action Action
		want   string
	}{
		{MouseAction{}, "mouse-none-x-0-y-0-wheel-0"},
		{MouseAction{Buttons: 7, X: -127, Y: 127, Wheel: -1}, "mouse-left-right-middle-x--127-y-127-wheel--1"},
		{MediaAction{Usage: 0xb6}, "media-prev-0x00b6"},
		{MediaAction{Usage: 0xcd}, "media-playpause-0x00cd"},
		{MediaAction{Usage: 1}, "media-0x0001"},
		{MediaAction{Usage: 0x23c}, "media-0x023c"},
		{MultiAction{Interval: 50, Repeat: 1, Keys: []byte{4, 0x68}}, "multi-2-keys-interval-50-repeat-1-17ebfb175afc"},
	} {
		c := actionConfiguration(t, tc.action)
		want := tc.want + "_rgb-cycle-slow-ffffff"
		for range 20 {
			if got := captureSettingsLabel(c[:]); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		}
	}
	for _, raw := range [][]byte{{1, 8}, {1, 0, 128}, {2, 0, 0}, {3, 0, 0, 1, 1, 4}} {
		c := syntheticSettings()
		copy(c[:], raw)
		if got := captureSettingsLabel(c[:]); !strings.HasPrefix(got, fmt.Sprintf("key-unknown-%x_", c[:5])) {
			t.Fatal("invalid action received a supported label", got)
		}
	}
}

func TestMultiCaptureLabelsBoundedAndDistinct(t *testing.T) {
	for _, count := range []int{1, 115} {
		c := actionConfiguration(t, MultiAction{Interval: 65535, Repeat: 255, Keys: bytes.Repeat([]byte{4}, count)})
		label := captureSettingsLabel(c[:])
		prefix := fmt.Sprintf("multi-%d-keys-interval-65535-repeat-255-", count)
		if !strings.HasPrefix(label, prefix) || len(label) != len(prefix)+12+len("_rgb-cycle-slow-ffffff") {
			t.Fatal("missing bounded sequence description or digest", label)
		}
		for _, offset := range []int{1, 2, 3, 4 + count} {
			changed := c
			changed[offset]--
			if offset == 4+count {
				changed[offset] = 5
			}
			if got := captureSettingsLabel(changed[:]); got == label {
				t.Fatal("active byte did not change label", offset, got)
			}
		}
		for offset := 5 + count; offset < 124; offset++ {
			changed := c
			changed[offset] ^= 1
			if got := captureSettingsLabel(changed[:]); got != label {
				t.Fatal("inactive byte changed label", offset, got)
			}
		}
		for _, attempt := range []int{1, 2, 10, 100} {
			name := captureCollisionName("260912-083853_"+label, attempt)
			match := labelledBackupNamePattern.FindStringSubmatch(name)
			if len(name) > 100 || len(match) != 5 || match[1] != "260912" || match[2] != "083853" {
				t.Fatal("unsafe name or changed capture groups", name, match)
			}
		}
	}
}

func TestTypedCaptureCollisionsAndOwnership(t *testing.T) {
	fixedCaptureTime(t)
	for _, action := range actionExamples()[1:] {
		t.Run(fmt.Sprintf("%T", action), func(t *testing.T) {
			root := t.TempDir()
			current := actionConfiguration(t, action)
			base := "260912-083853_" + captureSettingsLabel(current[:])
			var directories []string
			for attempt := 1; attempt <= 3; attempt++ {
				dir, err := newCapture(root, Candidate{})
				if err != nil {
					t.Fatal(err)
				}
				transport := newSettingsTransport()
				transport.current = current
				result, err := applySettingsWithCapture(transport, dir, settingsTarget(), ActionChanges{RGB: Changes{"red": 1}}, true, time.Second,
					func(configuration, configuration, string) (bool, error) { return true, nil }, captureNamedQueries)
				if err != nil || !result.ReadbackVerified || filepath.Base(result.Directory) != captureCollisionName(base, attempt) {
					t.Fatal(result, err)
				}
				_, captured, err := loadCapture(result.Directory)
				if err != nil || captured != current {
					t.Fatal("capture bytes changed", err)
				}
				if _, ok := classifyOwnedBackup(root, filepath.Base(result.Directory), "0112", "be077ba2"); !ok {
					t.Fatal("typed backup rejected by retention")
				}
				directories = append(directories, result.Directory)
			}
			lock, err := lockCaptureRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.close()
			if err := lock.retainBackups("be077ba2", 1); err != nil {
				t.Fatal(err)
			}
			for i, dir := range directories {
				_, err := os.Stat(dir)
				if i < 2 && !os.IsNotExist(err) || i == 2 && err != nil {
					t.Fatal("incorrect collision retention", dir, err)
				}
			}
		})
	}
}

func TestHistoricalCaptureNamesRetainOwnership(t *testing.T) {
	for _, name := range []string{
		"20260912-083853-abcd",
		"260912-083853_key-enter_rgb-cycle-slow-ffffff",
		"260912-083853_key-unknown_rgb-unknown-unknown",
		"260912-083853_key-unknown-0300320102_rgb-cycle-slow-ffffff-12",
	} {
		t.Run(name, func(t *testing.T) {
			created, ok := parseBackupName(name)
			if !ok || created.Format(time.RFC3339) != "2026-09-12T08:38:53Z" {
				t.Fatal("historical timestamp rejected", created)
			}
			root := t.TempDir()
			dir := filepath.Join(root, name)
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			result, err := applySettingsWithCapture(newSettingsTransport(), dir, settingsTarget(), Changes{"key": 0x68}, true, time.Second,
				func(configuration, configuration, string) (bool, error) { return true, nil }, captureQueries)
			if err != nil || result.Directory != dir {
				t.Fatal("historical name changed", result, err)
			}
			if _, ok := classifyOwnedBackup(root, name, "0112", "be077ba2"); !ok {
				t.Fatal("historical backup rejected by retention")
			}
			if _, ok := classifyOwnedBackup(root, name, "0112", "00000000"); ok {
				t.Fatal("wrong owner accepted")
			}
			if _, ok := classifyOwnedBackup(root, name, "0113", "be077ba2"); ok {
				t.Fatal("wrong model accepted")
			}
			lock, err := lockCaptureRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.close()
			if err := lock.retainBackups("be077ba2", 0); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("historical backup escaped retention", err)
			}
		})
	}
}
