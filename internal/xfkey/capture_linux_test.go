package xfkey

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
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
