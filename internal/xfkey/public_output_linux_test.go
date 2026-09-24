package xfkey

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestPublicSuccessfulWriteOutputFailure(t *testing.T) {
	for _, command := range []string{"media", "restore"} {
		for _, needle := range []string{"Settings saved.", "Warning:"} {
			for _, short := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/short=%t", command, needle, short), func(t *testing.T) {
					publicTestCaptureRoot(t)
					first := newSettingsTransport()
					original := first.current
					fresh := &backupCheckingTransport{settingsTransport: newSettingsTransport()}
					args := []string{"media", "playpause"}
					if command == "restore" {
						saved := newSettingsTransport()
						saved.current = actionConfiguration(t, MediaAction{Usage: 0xcd})
						args = []string{"restore", restoreTestSource(t, t.TempDir(), "source", saved)}
					}
					out := &failingHumanOutput{needle: needle, short: short}
					var result ApplyResult
					applies := 0
					access := publicActionTestAccess(t, first, func(candidate Candidate, root string, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
						applies++
						if out.failed {
							t.Fatal("output failed before apply")
						}
						dir, err := newCapture(root, candidate)
						if err != nil {
							return ApplyResult{}, err
						}
						fresh.dir = dir
						result, err = applySettingsConfirmed(fresh, dir, target, changes, true, time.Second, guard, true)
						if err != nil || !result.ReadbackVerified {
							t.Fatal("synthetic write failed", result, err)
						}
						if needle == "Warning:" {
							result.CleanupWarning = "synthetic cleanup warning"
						}
						return result, nil
					})
					err := runPublic(args, &cliRuntime{strings.NewReader("yes\n"), out, io.Discard}, access)
					want := io.ErrClosedPipe
					if short {
						want = io.ErrShortWrite
					}
					if !errors.Is(err, want) || ExitCode(err) != 1 || !out.failed || applies != 1 {
						t.Fatal("post-write output failure lost or write repeated", err, applies)
					}
					if !fresh.checked || !result.WriteAttempted || !result.ReadbackVerified || len(fresh.packets) != 11 {
						t.Fatal("write did not complete before output failure", result)
					}
					action, err := decodeAction(fresh.current)
					if err != nil || action != (MediaAction{Usage: 0xcd}) {
						t.Fatal("saved action changed after output failure", action, err)
					}
					_, backup, err := loadCapture(fresh.dir)
					if err != nil || backup != original {
						t.Fatal("backup changed after output failure", err)
					}
				})
			}
		}
	}
}
