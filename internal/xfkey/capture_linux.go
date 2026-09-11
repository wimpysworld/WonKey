package xfkey

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CaptureResult struct {
	Directory string         `json:"capture_directory"`
	Identity  deviceIdentity `json:"identity"`
	Readback  *Readback      `json:"readback,omitempty"`
	KeyPrefix string         `json:"key_prefix_hex,omitempty"`
	Warning   string         `json:"warning"`
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func saveExclusive(dir, name string, data []byte) error {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = fmt.Errorf("short capture write")
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return syncDir(dir)
}

func saveCompletion(dir string, data []byte) error {
	const pending = ".result.json.pending"
	if err := saveExclusive(dir, pending, data); err != nil {
		return err
	}
	marker := filepath.Join(dir, "result.json")
	// Linking publishes only synchronised contents and never replaces an existing marker.
	if err := os.Link(filepath.Join(dir, pending), marker); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		removeErr := os.Remove(marker)
		return errors.Join(err, removeErr, syncDir(dir))
	}
	return nil
}

func newCapture(root string, target Candidate) (string, error) {
	return newCaptureWithSync(root, target, syncDir)
}

func newCaptureWithSync(root string, target Candidate, syncDirectory func(string) error) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("--capture-root must be an absolute directory")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	// Existing ancestors can also be newly created by a caller or an interrupted attempt.
	// Sync the resolved chain bottom-up so each directory name is durable in its parent.
	for path := root; ; path = filepath.Dir(path) {
		if err := syncDirectory(path); err != nil {
			return "", fmt.Errorf("synchronise capture ancestor %s: %w", path, err)
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	dir, err := os.MkdirTemp(root, time.Now().UTC().Format("20060102T150405.000000000Z")+"-")
	if err != nil {
		return "", err
	}
	if err := syncDirectory(root); err != nil {
		return dir, err
	}
	metadata := struct {
		Timestamp string    `json:"started_utc"`
		Target    Candidate `json:"target"`
		Warning   string    `json:"warning"`
	}{time.Now().UTC().Format(time.RFC3339Nano), target, "Query capture only. Not a verified restore image. Missing result.json means incomplete or failed."}
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return dir, err
	}
	return dir, saveExclusive(dir, "provenance.json", b)
}

func captureQueries(t queryTransport, dir string, readback bool) (CaptureResult, error) {
	result := CaptureResult{Directory: dir, Warning: "Current query replies only, not a verified restore image. Field meanings are host-derived."}
	commands := []byte{1}
	if readback {
		commands = append(commands, 6, 7, 8)
	}
	var replies [3][]byte
	for i, command := range commands {
		reply, err := query(t, command)
		if len(reply) > 0 {
			if saveErr := saveExclusive(dir, fmt.Sprintf("reply-%02x.bin", command), reply); saveErr != nil {
				return result, saveErr
			}
		}
		if err != nil {
			return result, fmt.Errorf("AF %02x: %w", command, err)
		}
		if i == 0 {
			result.Identity, err = parseIdentity(reply)
			if err != nil {
				return result, err
			}
		} else {
			replies[i-1] = reply
		}
	}
	if readback {
		parsed, err := parseReadback(replies)
		if err != nil {
			return result, err
		}
		config, err := hex.DecodeString(parsed.Configuration)
		if err != nil {
			return result, err
		}
		if err := saveExclusive(dir, "configuration.bin", config); err != nil {
			return result, err
		}
		result.Readback = &parsed
		result.KeyPrefix = hex.EncodeToString(config[:5])
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	return result, saveCompletion(dir, b)
}

func liveCapture(path, root string, readback bool) (CaptureResult, error) {
	var result CaptureResult
	candidates, err := discover("/sys/bus/usb/devices", "/dev")
	if err != nil {
		return result, err
	}
	target, err := selectCandidate(candidates, path)
	if err != nil {
		return result, err
	}
	dir, err := newCapture(root, *target)
	result.Directory = dir
	if err != nil {
		return result, err
	}
	t, err := openTarget(*target)
	if err != nil {
		return result, fmt.Errorf("capture %s: %w", dir, err)
	}
	defer t.Close()
	result, err = captureQueries(t, dir, readback)
	if err != nil {
		return result, fmt.Errorf("capture %s incomplete: %w; stop without retry", dir, err)
	}
	return result, nil
}
