package xfkey

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type consumedReplyTransport struct {
	fakeTransport
	expire bool
}

func (f *consumedReplyTransport) Read(b []byte) (int, error) {
	n, err := f.fakeTransport.Read(b)
	if f.expire {
		time.Sleep(time.Until(f.deadlines[len(f.deadlines)-1]) + time.Millisecond)
		return n, err
	}
	return n, io.ErrUnexpectedEOF
}

type closeRecorder struct {
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

func TestShowQueriesInMemoryAndCreatesNothing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "must-not-exist")
	t.Setenv("XFKEY_CAPTURE_ROOT", root)
	transport := newSettingsTransport()
	closer := &closeRecorder{}
	oldDiscover, oldOpen := queryDiscover, queryOpenTarget
	t.Cleanup(func() { queryDiscover, queryOpenTarget = oldDiscover, oldOpen })
	queryDiscover = func(sysfsRoot, devRoot string) ([]Candidate, error) {
		if sysfsRoot != "/sys/bus/usb/devices" || devRoot != "/dev" {
			t.Fatalf("discovery roots = %q, %q", sysfsRoot, devRoot)
		}
		return []Candidate{{PhysicalPath: "1-2.3", Compatible: true}}, nil
	}
	queryOpenTarget = func(target Candidate) (queryTransport, io.Closer, error) {
		if target.PhysicalPath != "1-2.3" {
			t.Fatalf("opened target = %#v", target)
		}
		return transport, closer, nil
	}
	var stdout bytes.Buffer
	command := showCommand{Device: "1-2.3"}
	if err := command.Run(&cliRuntime{out: &stdout, errOut: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]byte{1, 6, 7, 8}, packetCommands(transport.packets)) || !closer.closed {
		t.Fatalf("commands=%v closed=%t", packetCommands(transport.packets), closer.closed)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("show created the configured root: %v", err)
	}
}

func packetCommands(packets []vendorPacket) []byte {
	commands := make([]byte, len(packets))
	for i := range packets {
		commands[i] = packets[i][1]
	}
	return commands
}

func TestShortCaptureNamesAreExclusive(t *testing.T) {
	oldNow, oldRandom := captureNow, captureRandom
	t.Cleanup(func() { captureNow, captureRandom = oldNow, oldRandom })
	captureNow = func() time.Time { return time.Date(2026, 9, 11, 12, 34, 56, 0, time.UTC) }
	values := [][]byte{{0xa3, 0xf2}, {0xa3, 0xf2}, {0xbe, 0xef}}
	captureRandom = func(out []byte) (int, error) {
		copy(out, values[0])
		values = values[1:]
		return len(out), nil
	}
	root := t.TempDir()
	first, err := newCapture(root, Candidate{})
	if err != nil || filepath.Base(first) != "20260911-123456-a3f2" {
		t.Fatalf("first=%q error=%v", first, err)
	}
	second, err := newCapture(root, Candidate{})
	if err != nil || filepath.Base(second) != "20260911-123456-beef" {
		t.Fatalf("second=%q error=%v", second, err)
	}
	if _, ok := parseBackupName(filepath.Base(first)); !ok {
		t.Fatal("generated name does not match the exact grammar")
	}
}

func TestRetentionKeepsNewestTenPerDevice(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	create := func(index int, identifier string) string {
		name := fmt.Sprintf("20260911-123456-%04x", index)
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := saveExclusive(dir, "provenance.json", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		transport := newSettingsTransport()
		target := settingsTarget()
		if identifier != target.Identifier {
			raw, _ := hex.DecodeString(identifier)
			copy(transport.identity[6:10], raw)
			target.Identifier = identifier
		}
		if _, err := applySettings(transport, dir, target, Changes{"key": 0x28}, true, time.Second); err != nil {
			t.Fatal(err)
		}
		return name
	}
	for i := 0; i < 12; i++ {
		create(i, "be077ba2")
	}
	for i := 100; i < 111; i++ {
		create(i, "01020304")
	}
	lock, err := lockCaptureRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.retainBackups("0112", "be077ba2", 10); err != nil {
		t.Fatal(err)
	}
	if err := lock.retainBackups("0112", "01020304", 10); err != nil {
		t.Fatal(err)
	}
	if err := lock.close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	countA, countB := 0, 0
	for _, entry := range entries {
		if backup, ok := classifyOwnedBackup(root, entry.Name(), "0112", "be077ba2"); ok && backup.name != "" {
			countA++
		}
		if backup, ok := classifyOwnedBackup(root, entry.Name(), "0112", "01020304"); ok && backup.name != "" {
			countB++
		}
	}
	if countA != 10 || countB != 10 {
		t.Fatalf("retained A=%d B=%d", countA, countB)
	}
	for _, removed := range []string{"20260911-123456-0000", "20260911-123456-0001", "20260911-123456-0064"} {
		if _, err := os.Lstat(filepath.Join(root, removed)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("old backup remains: %s", removed)
		}
	}
}

func createOwnedTestBackup(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveExclusive(dir, "provenance.json", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := applySettings(newSettingsTransport(), dir, settingsTarget(), Changes{"key": 0x28}, true, time.Second); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRetentionRejectsSymlinksAndIncompleteOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"symlink", func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "plan.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("result.json", filepath.Join(dir, "plan.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"incomplete outcome", func(t *testing.T, dir string) {
			outcome := fmt.Sprintf(`{"capture_directory":%q,"outcome":"no-op"}`, dir)
			if err := os.WriteFile(filepath.Join(dir, "apply-outcome.json"), []byte(outcome), 0600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			name := "20260911-123456-0000"
			dir := createOwnedTestBackup(t, root, name)
			tc.mutate(t, dir)
			lock, err := lockCaptureRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := lock.retainBackups("0112", "be077ba2", 0); err != nil {
				t.Fatal(err)
			}
			if err := lock.close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(dir); err != nil {
				t.Fatalf("retention changed rejected backup: %v", err)
			}
		})
	}
}

func TestRetentionRevalidatesAfterRename(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name := "20260911-123456-0000"
	createOwnedTestBackup(t, root, name)
	lock, err := lockCaptureRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.close()
	backup, _, ok := classifyOwnedBackupAt(lock.rootFD, lock.root, name, "0112", "be077ba2")
	if !ok {
		t.Fatal("valid backup was not classified")
	}
	oldRename := renameBackup
	t.Cleanup(func() { renameBackup = oldRename })
	renameBackup = func(oldDirFD int, oldPath string, newDirFD int, newPath string, flags uint) error {
		if err := unix.Renameat2(oldDirFD, oldPath, newDirFD, newPath, flags); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, newPath, "unexpected"), []byte("foreign"), 0600)
	}
	if err := lock.removeBackup(backup, "0112", "be077ba2"); err == nil {
		t.Fatal("removed a backup that changed after rename")
	}
	if data, err := os.ReadFile(filepath.Join(root, ".wonkey-remove-"+name, "unexpected")); err != nil || string(data) != "foreign" {
		t.Fatalf("unexpected entry was changed: %q, %v", data, err)
	}
}

func TestCapturePreservesConsumedReplyOnFailure(t *testing.T) {
	for _, expire := range []bool{true, false} {
		t.Run(map[bool]string{true: "deadline", false: "read error"}[expire], func(t *testing.T) {
			replies := validReplies()
			want := append([]byte(nil), replies[0]...)
			f := &consumedReplyTransport{fakeTransport: fakeTransport{replies: replies}, expire: expire}
			dir := t.TempDir()
			_, err := captureQueries(f, dir, true)
			wantErr := io.ErrUnexpectedEOF
			if expire {
				wantErr = os.ErrDeadlineExceeded
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("got %v, want %v", err, wantErr)
			}
			got, err := os.ReadFile(filepath.Join(dir, "reply-01.bin"))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("consumed reply not preserved: %x, %v", got, err)
			}
			if !reflect.DeepEqual(f.commands, []byte{1}) {
				t.Fatalf("continued after failure: %x", f.commands)
			}
			if _, err := os.Stat(filepath.Join(dir, "result.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed capture has completion marker: %v", err)
			}
		})
	}
}

func TestCompletionPublication(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"complete":true}`)
	if err := saveCompletion(dir, data); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "result.json")
	got, err := os.ReadFile(marker)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal(string(got), err)
	}
	info, err := os.Stat(marker)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}

	other := t.TempDir()
	if err := saveExclusive(other, "result.json", data); err != nil {
		t.Fatal(err)
	}
	if err := saveCompletion(other, []byte("replacement")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("replaced existing marker: %v", err)
	}
	got, err = os.ReadFile(filepath.Join(other, "result.json"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("existing marker changed", err)
	}
}

func TestFailedCompletionWriteLeavesNoMarker(t *testing.T) {
	if os.Getenv("WONKEY_TEST_FILE_LIMIT") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFailedCompletionWriteLeavesNoMarker$")
		cmd.Env = append(os.Environ(), "WONKEY_TEST_FILE_LIMIT=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated file-limit test: %v\n%s", err, output)
		}
		return
	}
	dir := t.TempDir()
	raw := []byte("existing reply")
	if err := saveExclusive(dir, "reply-01.bin", raw); err != nil {
		t.Fatal(err)
	}
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
		t.Fatal(err)
	}
	limited := limit
	limited.Cur = 16
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	err := saveCompletion(dir, bytes.Repeat([]byte("x"), 128))
	if restoreErr := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if err == nil {
		t.Fatal("accepted failed completion write")
	}
	if _, err := os.Stat(filepath.Join(dir, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed completion write retained marker: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "reply-01.bin"))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("existing reply changed", err)
	}
	partial, err := os.ReadFile(filepath.Join(dir, ".result.json.pending"))
	if err != nil || len(partial) != 16 {
		t.Fatalf("partial staging file not preserved: %d bytes, %v", len(partial), err)
	}
}
