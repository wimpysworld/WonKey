package xfkey

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCapturedSettingsLabels(t *testing.T) {
	for key, value := range keyValues {
		for mask := 0; mask < 16; mask++ {
			for mode, index := range lightingValues {
				c := syntheticSettings()
				c[4], c[2], c[124] = byte(value), byte(mask), byte(index+1)
				copy(c[125:], []byte{0, 0, 255})
				combination := key
				if mask != 0 {
					combination = strings.ReplaceAll(modifierName(byte(mask)), ",", "-") + "-" + key
				}
				want := "key-" + combination + "_rgb-" + mode + "-0000ff"
				if got := captureSettingsLabel(c[:]); got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			}
		}
	}
	for _, tc := range []struct {
		offset int
		value  byte
		want   string
	}{
		{0, 1, "key-unknown-0101000128_rgb-gradient-ffffff"},
		{1, 0, "key-unknown-0000000128_rgb-gradient-ffffff"},
		{2, 128, "key-unknown-0001800128_rgb-gradient-ffffff"},
		{3, 2, "key-unknown-0001000228_rgb-gradient-ffffff"},
		{4, 0x2f, "key-unknown-000100012f_rgb-gradient-ffffff"},
		{124, 255, "key-enter_rgb-unknown-ff-ffffff"},
	} {
		c := syntheticSettings()
		c[tc.offset] = tc.value
		if got := captureSettingsLabel(c[:]); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
	if got := captureSettingsLabel(nil); got != "key-unknown_rgb-unknown-unknown" {
		t.Fatal(got)
	}
}

func fixedCaptureTime(t *testing.T) {
	t.Helper()
	old := captureNow
	t.Cleanup(func() { captureNow = old })
	captureNow = func() time.Time { return time.Date(2026, 9, 12, 8, 38, 53, 0, time.UTC) }
}

func TestNamedCaptureCollisionsAndEvidence(t *testing.T) {
	fixedCaptureTime(t)
	root := t.TempDir()
	for attempt := 1; attempt <= 3; attempt++ {
		dir, err := newCapture(root, Candidate{})
		if err != nil {
			t.Fatal(err)
		}
		transport := newSettingsTransport()
		transport.current[2], transport.current[4], transport.current[124] = 13, 0x68, 2
		copy(transport.current[125:], []byte{0, 0, 255})
		result, err := captureNamedQueries(transport, dir, true)
		want := captureCollisionName("260912-083853_key-ctrl-alt-super-f13_rgb-steady-0000ff", attempt)
		if err != nil || filepath.Base(result.Directory) != want {
			t.Fatalf("result=%+v err=%v want=%s", result, err, want)
		}
		_, config, err := loadCapture(result.Directory)
		if err != nil || config != transport.current {
			t.Fatal("captured bytes changed", err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 3 {
		t.Fatal(entries, err)
	}
}

func TestNamedApplyUsesCapturedNotProposedSettings(t *testing.T) {
	fixedCaptureTime(t)
	dir := settingsDir(t)
	transport := newSettingsTransport()
	current := transport.current
	result, err := applySettingsWithCapture(transport, dir, settingsTarget(), Changes{"key": 0x68, "rgb-mode": 1, "red": 0, "green": 0, "blue": 255}, true, time.Second,
		func(configuration, configuration, string) (bool, error) { return true, nil }, captureNamedQueries)
	if err != nil || !result.ReadbackVerified || filepath.Base(result.Directory) != "260912-083853_key-enter_rgb-gradient-ffffff" {
		t.Fatal(result, err)
	}
	_, captured, err := loadCapture(result.Directory)
	if err != nil || captured != current {
		t.Fatal("backup no longer matches captured settings", err)
	}
	checkOutcome(t, result.Directory, result)
	if _, ok := classifyOwnedBackup(filepath.Dir(result.Directory), filepath.Base(result.Directory), "0112", "be077ba2"); !ok {
		t.Fatal("new backup rejected by retention")
	}
}

func TestNamingFailurePreservesRepliesAndStopsUpload(t *testing.T) {
	fixedCaptureTime(t)
	old := renameCapture
	t.Cleanup(func() { renameCapture = old })
	renameCapture = func(int, string, int, string, uint) error { return unix.EIO }
	dir := settingsDir(t)
	transport := newSettingsTransport()
	result, err := applySettingsWithCapture(transport, dir, settingsTarget(), Changes{"key": 0x68}, true, time.Second,
		func(configuration, configuration, string) (bool, error) { return true, nil }, captureNamedQueries)
	if !errors.Is(err, unix.EIO) || result.Directory != dir {
		t.Fatal(result, err)
	}
	checkNoUpload(t, transport)
	checkOutcome(t, dir, result)
	if raw, err := os.ReadFile(filepath.Join(dir, "reply-01.bin")); err != nil || !bytes.Equal(raw, transport.identity) {
		t.Fatal("lost query evidence", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed naming published completion", err)
	}
}

func TestNamedCaptureUnknownAndIdentityOnly(t *testing.T) {
	for _, readback := range []bool{true, false} {
		t.Run(fmt.Sprint(readback), func(t *testing.T) {
			dir := settingsDir(t)
			transport := newSettingsTransport()
			transport.current[4], transport.current[124] = 0xff, 0xff
			result, err := captureNamedQueries(transport, dir, readback)
			if err != nil {
				t.Fatal(err)
			}
			want := "_key-unknown_rgb-unknown-unknown"
			if readback {
				want = "_key-unknown-00010001ff_rgb-unknown-ff-ffffff"
				if _, _, err := loadCapture(result.Directory); err != nil {
					t.Fatal(err)
				}
			}
			if !strings.HasSuffix(result.Directory, want) {
				t.Fatal(result.Directory)
			}
		})
	}
}

func TestNamedCaptureRetentionOrdersCollisionNumbers(t *testing.T) {
	fixedCaptureTime(t)
	root := t.TempDir()
	var directories []string
	for i := 0; i < 12; i++ {
		dir, err := newCapture(root, Candidate{})
		if err != nil {
			t.Fatal(err)
		}
		result, err := applySettingsWithCapture(newSettingsTransport(), dir, settingsTarget(), Changes{"key": 0x28}, true, time.Second,
			func(configuration, configuration, string) (bool, error) { return true, nil }, captureNamedQueries)
		if err != nil {
			t.Fatal(err)
		}
		directories = append(directories, result.Directory)
	}
	lock, err := lockCaptureRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.close()
	if err := lock.retainBackups("0112", "be077ba2", 10); err != nil {
		t.Fatal(err)
	}
	for i, dir := range directories {
		_, err := os.Stat(dir)
		if i < 2 && !errors.Is(err, os.ErrNotExist) || i >= 2 && err != nil {
			t.Fatalf("backup %d: %v", i+1, err)
		}
	}
}

func TestCaptureCollisionExhaustionDoesNotOverwrite(t *testing.T) {
	fixedCaptureTime(t)
	root := t.TempDir()
	base := "260912-083853_key-unknown_rgb-unknown-unknown"
	for attempt := 1; attempt <= 100; attempt++ {
		if err := os.WriteFile(filepath.Join(root, captureCollisionName(base, attempt)), []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if dir, err := newCapture(root, Candidate{}); err == nil || dir != "" {
		t.Fatal(dir, err)
	}
	for attempt := 1; attempt <= 100; attempt++ {
		raw, err := os.ReadFile(filepath.Join(root, captureCollisionName(base, attempt)))
		if err != nil || string(raw) != "existing" {
			t.Fatal("overwrote existing entry", err)
		}
	}
}
