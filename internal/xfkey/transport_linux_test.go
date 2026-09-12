package xfkey

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type fakeTransport struct {
	commands   []byte
	replies    [][]byte
	waits      []bool
	deadlines  []time.Time
	failWait   int
	shortWrite bool
	invalid    bool
}

func (f *fakeTransport) Validate() error {
	if f.invalid {
		return fmt.Errorf("identity changed")
	}
	return nil
}
func (f *fakeTransport) Wait(write bool, d time.Time) error {
	f.waits = append(f.waits, write)
	f.deadlines = append(f.deadlines, d)
	if f.failWait == len(f.waits) {
		return os.ErrDeadlineExceeded
	}
	return nil
}
func (f *fakeTransport) StartWrite(b []byte, _ time.Time) (<-chan writeResult, error) {
	n, err := f.Write(b)
	result := make(chan writeResult, 1)
	result <- writeResult{n, err}
	return result, nil
}
func (f *fakeTransport) Write(b []byte) (int, error) {
	if len(b) != 65 || b[0] != 0 || b[1] != 0xaf || !bytes.Equal(b[3:], make([]byte, 62)) {
		return 0, fmt.Errorf("unexpected packet %x", b)
	}
	f.commands = append(f.commands, b[2])
	if f.shortWrite {
		return 64, nil
	}
	return 65, nil
}
func (f *fakeTransport) Read(b []byte) (int, error) {
	if len(f.replies) == 0 {
		return 0, io.EOF
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	return copy(b, reply), nil
}
func validReplies() [][]byte {
	identify := syntheticReply(1)
	identify[2], identify[3] = 1, 0x12
	return [][]byte{identify, syntheticReply(6), syntheticReply(7), syntheticReply(8)}
}

func TestCaptureSequenceAndExclusiveFiles(t *testing.T) {
	root := t.TempDir()
	dir, err := newCapture(root, Candidate{PhysicalPath: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeTransport{replies: validReplies()}
	result, err := captureQueries(f, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.commands, []byte{1, 6, 7, 8}) {
		t.Fatal(f.commands)
	}
	for i := 0; i < 8; i += 2 {
		if !f.waits[i] || f.waits[i+1] || !f.deadlines[i].Equal(f.deadlines[i+1]) {
			t.Fatal("transaction must share one deadline")
		}
	}
	if result.Readback == nil {
		t.Fatal("missing readback")
	}
	for _, name := range []string{"reply-01.bin", "reply-06.bin", "reply-07.bin", "reply-08.bin", "configuration.bin"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		want := 64
		if name == "configuration.bin" {
			want = 128
		}
		if len(data) != want {
			t.Fatal(name, len(data))
		}
		if err := saveExclusive(dir, name, []byte("overwrite")); !errors.Is(err, os.ErrExist) {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(filepath.Join(dir, name))
		if !bytes.Equal(data, after) {
			t.Fatal("capture changed")
		}
	}
	second, err := newCapture(root, Candidate{})
	if err != nil || second == dir {
		t.Fatal(second, err)
	}
}

func TestCaptureStopsOnBadReplies(t *testing.T) {
	for index := 0; index < 4; index++ {
		for _, kind := range []string{"short", "long", "header", "prefix", "timeout"} {
			t.Run(fmt.Sprintf("%d-%s", index, kind), func(t *testing.T) {
				replies := validReplies()
				f := &fakeTransport{replies: replies}
				switch kind {
				case "short":
					replies[index] = replies[index][:63]
				case "long":
					replies[index] = append(replies[index], 0)
				case "header":
					replies[index][1] = 0xff
				case "prefix":
					replies[index][0] = 0
				case "timeout":
					f.failWait = index*2 + 2
				}
				dir := t.TempDir()
				if _, err := captureQueries(f, dir, true); err == nil {
					t.Fatal("accepted invalid reply")
				}
				if len(f.commands) != index+1 {
					t.Fatal("continued after failure", f.commands)
				}
				if _, err := os.Stat(filepath.Join(dir, "result.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed capture marked complete")
				}
			})
		}
	}
	f := &fakeTransport{replies: validReplies()}
	f.replies[0][2], f.replies[0][3] = 0x12, 1
	if _, err := captureQueries(f, t.TempDir(), true); err == nil || len(f.commands) != 1 {
		t.Fatal("model gate failed")
	}
}

func TestQueryFailurePaths(t *testing.T) {
	for name, f := range map[string]*fakeTransport{
		"guard": {invalid: true}, "write timeout": {failWait: 1}, "short write": {shortWrite: true}, "read timeout": {failWait: 2}, "read error": {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := query(f, 1); err == nil {
				t.Fatal("accepted failure")
			}
			if len(f.commands) > 1 {
				t.Fatal("retried")
			}
		})
	}
	for _, cmd := range []byte{0, 2, 4, 5, 9, 255} {
		f := &fakeTransport{}
		if _, err := query(f, cmd); err == nil || len(f.commands) != 0 || len(f.waits) != 0 {
			t.Fatal("unsafe command", cmd)
		}
	}
	f := &fakeTransport{replies: validReplies()}
	if _, err := captureQueries(f, t.TempDir(), false); err != nil || !reflect.DeepEqual(f.commands, []byte{1}) {
		t.Fatal(f.commands, err)
	}
}

func TestNonblockingWaitTimeoutOffline(t *testing.T) {
	var pipes [2]int
	if err := syscall.Pipe2(pipes[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(pipes[0])
	defer syscall.Close(pipes[1])
	transport := &hidrawTransport{fd: pipes[0]}
	start := time.Now()
	if err := transport.Wait(false, start.Add(10*time.Millisecond)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("wait not bounded")
	}
	if err := transport.Wait(false, time.Now().Add(-time.Second)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestRemovedLiveCLIRejectedOffline(t *testing.T) {
	for _, cmd := range []string{"identify", "readback"} {
		for _, args := range [][]string{{cmd}, {cmd, "--yes"}, {cmd, "--packet", "af04"}, {cmd, "unexpected"}} {
			if err := Run(args, io.Discard, io.Discard); err == nil {
				t.Fatal(args)
			}
		}
	}
}

type stalledWriter struct{ fakeTransport }

func (f *stalledWriter) StartWrite(b []byte, _ time.Time) (<-chan writeResult, error) {
	f.commands = append(f.commands, b[2])
	return make(chan writeResult), nil
}

func TestSubmittedWriteTimeoutStopsSession(t *testing.T) {
	f := &stalledWriter{}
	start := time.Now()
	if _, err := queryWithTimeout(f, 1, 10*time.Millisecond); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("write wait not bounded")
	}
	if !reflect.DeepEqual(f.commands, []byte{1}) || !reflect.DeepEqual(f.waits, []bool{true}) {
		t.Fatal("continued after write timeout")
	}
}
