package xfkey

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type ApplyTarget struct {
	Identifier string `json:"identifier"`
	Version    string `json:"version_hex"`
}

func (target ApplyTarget) validate() error {
	for _, field := range []struct {
		value string
		size  int
	}{{target.Identifier, 8}, {target.Version, 4}} {
		if len(field.value) != field.size {
			return fmt.Errorf("--expect-identifier (8 hex digits) and --expect-version (4 hex digits) are required")
		}
		if _, err := hex.DecodeString(field.value); err != nil {
			return err
		}
	}
	return nil
}

func (target ApplyTarget) matches(identity deviceIdentity) bool {
	id, _ := hex.DecodeString(target.Identifier)
	version, _ := hex.DecodeString(target.Version)
	return identity.Model == 0x0112 && identity.Identifier == hex.EncodeToString(id) && identity.Version == uint16(version[0])<<8|uint16(version[1])
}

type submissionTrackingTransport struct {
	queryTransport
	submitted *bool
}

func (t submissionTrackingTransport) StartWrite(b []byte, deadline time.Time) (<-chan writeResult, error) {
	result, err := t.queryTransport.StartWrite(b, deadline)
	if err == nil {
		*t.submitted = true
	}
	return result, err
}

type ApplyResult struct {
	Directory           string       `json:"capture_directory"`
	Outcome             string       `json:"outcome"`
	Changes             []changeView `json:"changes"`
	CommitEcho          bool         `json:"commit_echo_received"`
	ReadbackVerified    bool         `json:"all_128_bytes_readback_verified"`
	WriteAttempted      bool         `json:"write_attempted"`
	PersistenceVerified bool         `json:"persistence_after_reconnect_verified"`
	Error               string       `json:"error,omitempty"`
	Warning             string       `json:"warning"`
	CleanupWarning      string       `json:"cleanup_warning,omitempty"`
}

// This transaction is the only live configuration path. Query permissions stay unchanged.
func applySettings(t queryTransport, dir string, target ApplyTarget, changes Changes, write bool, timeout time.Duration) (ApplyResult, error) {
	return applySettingsConfirmed(t, dir, target, changes, write, timeout, func(configuration, configuration, string) (bool, error) { return true, nil })
}

func applySettingsConfirmed(t queryTransport, dir string, target ApplyTarget, changes Changes, write bool, timeout time.Duration, confirm func(configuration, configuration, string) (bool, error), checkNoOp ...bool) (result ApplyResult, err error) {
	return applySettingsWithCapture(t, dir, target, changes, write, timeout, confirm, captureQueries, checkNoOp...)
}

func applySettingsWithCapture(t queryTransport, dir string, target ApplyTarget, changes Changes, write bool, timeout time.Duration, confirm func(configuration, configuration, string) (bool, error), capture func(queryTransport, string, bool) (CaptureResult, error), checkNoOp ...bool) (result ApplyResult, err error) {
	result = ApplyResult{Directory: dir, Outcome: "failed-before-upload", Warning: "No automatic retry or rollback. Readback verifies current state only, not persistence after reconnect. A timed-out submitted write can still complete in the kernel."}
	if !write {
		return result, errWriteRequired
	}
	if err = target.validate(); err != nil {
		return result, err
	}
	if err = changes.validate(); err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			result.Error = err.Error()
		}
		b, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr == nil {
			marshalErr = saveExclusive(dir, "apply-outcome.json", b)
		}
		if marshalErr != nil {
			err = errors.Join(err, fmt.Errorf("outcome storage failed: %w; device state must not be assumed", marshalErr))
		}
	}()
	backupCapture, captureErr := capture(t, dir, true)
	dir = backupCapture.Directory
	result.Directory = dir
	if captureErr != nil {
		return result, captureErr
	}
	// Reopen the durable backup and validate it before permitting any configuration write.
	backup, current, err := loadCapture(dir)
	if err != nil {
		return result, err
	}
	if !target.matches(backup.Identity) {
		return result, fmt.Errorf("current identity differs from explicitly expected target")
	}
	if err = saveBackupRecord(dir, backup.Identity); err != nil {
		return result, err
	}
	intended, err := changeConfiguration(current, changes)
	if err != nil {
		return result, err
	}
	result.Changes = settingViews(current, intended)
	plan := settingsPlan(current, intended)
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return result, err
	}
	if err = saveExclusive(dir, "plan.json", b); err != nil {
		return result, err
	}
	if err = saveExclusive(dir, "intended-configuration.bin", intended[:]); err != nil {
		return result, err
	}
	if len(plan.ChangedOffsets) == 0 && (len(checkNoOp) == 0 || !checkNoOp[0]) {
		result.Outcome = "no-op"
		return result, nil
	}
	confirmed, confirmErr := confirm(current, intended, dir)
	if confirmErr != nil {
		return result, confirmErr
	}
	if !confirmed {
		result.Outcome = "cancelled"
		return result, nil
	}
	if len(plan.ChangedOffsets) == 0 {
		result.Outcome = "no-op"
		return result, nil
	}
	packets := preview(intended)
	for i, packet := range packets[1:] {
		name := fmt.Sprintf("write-%d", i+1)
		if err = saveExclusive(dir, name+"-request.bin", packet[:]); err != nil {
			return result, err
		}
		transport := t
		if i == 0 {
			transport = submissionTrackingTransport{queryTransport: t, submitted: &result.WriteAttempted}
		}
		reply, exchangeErr := exchange(transport, packet, timeout)
		if result.WriteAttempted {
			result.Outcome = "failed-state-unknown"
		}
		if len(reply) > 0 {
			if err = saveExclusive(dir, name+"-reply.bin", reply); err != nil {
				return result, errors.Join(exchangeErr, err)
			}
		}
		if exchangeErr != nil {
			return result, fmt.Errorf("%s: %w; no further commands", name, exchangeErr)
		}
		if err = validateEcho(packet, reply); err != nil {
			return result, fmt.Errorf("%s: %w", name, err)
		}
	}
	result.CommitEcho = true
	err = verifyAppliedSettings(t, dir, intended, timeout, &result)
	return result, err
}

func verifyAppliedSettings(t queryTransport, dir string, intended configuration, timeout time.Duration, result *ApplyResult) error {
	var replies [3][]byte
	for i := range replies {
		reply, queryErr := queryWithTimeout(t, byte(6+i), timeout)
		if len(reply) > 0 {
			if err := saveExclusive(dir, fmt.Sprintf("post-reply-%02x.bin", i+6), reply); err != nil {
				return errors.Join(queryErr, err)
			}
		}
		if queryErr != nil {
			return fmt.Errorf("post-commit readback: %w; no rollback", queryErr)
		}
		replies[i] = reply
	}
	observed, err := parseReadback(replies)
	if err != nil {
		return err
	}
	raw, _ := hex.DecodeString(observed.Configuration)
	if err = saveExclusive(dir, "post-configuration.bin", raw); err != nil {
		return err
	}
	if observed.Configuration != hex.EncodeToString(intended[:]) {
		result.Outcome = "readback-mismatch"
		return fmt.Errorf("post-commit configuration differs from intended bytes; no rollback, persistence unverified")
	}
	result.Outcome = "readback-verified"
	result.ReadbackVerified = true
	return nil
}

func liveApplyBound(path, root string, target ApplyTarget, changes Changes, write bool, confirm func(configuration, configuration, string) (bool, error), pinned *Candidate, checkNoOp ...bool) (result ApplyResult, err error) {
	if !write {
		return result, errWriteRequired
	}
	if path == "" || root == "" {
		return result, fmt.Errorf("apply requires explicit --path and --capture-root")
	}
	if err := target.validate(); err != nil {
		return result, err
	}
	if err := changes.validate(); err != nil {
		return result, err
	}
	lifecycle, err := lockCaptureRoot(root)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := lifecycle.close(); closeErr != nil && err == nil {
			warning := fmt.Sprintf("release backup lifecycle lock: %v", closeErr)
			if result.CleanupWarning != "" {
				warning = result.CleanupWarning + "; " + warning
			}
			result.CleanupWarning = warning
		}
	}()
	candidates, err := liveDiscover("/sys/bus/usb/devices", "/dev")
	if err != nil {
		return result, err
	}
	selected, err := selectCandidate(candidates, path)
	if err != nil {
		return result, err
	}
	if pinned != nil && !sameCandidate(*selected, *pinned) {
		return result, fmt.Errorf("selected descriptor, path or node changed; no settings write sent")
	}
	dir, err := newCaptureInRoot(lifecycle.root, *selected, syncDir)
	result.Directory = dir
	if err != nil {
		return result, err
	}
	t, closer, err := liveOpenTarget(*selected)
	if err != nil {
		return result, fmt.Errorf("backup %s: %w", dir, err)
	}
	defer closer.Close()
	result, err = applySettingsWithCapture(t, dir, target, changes, write, transactionTimeout, confirm, captureNamedQueries, checkNoOp...)
	if err != nil {
		return result, fmt.Errorf("backup/transaction %s: %w", result.Directory, err)
	}
	if result.Outcome == "no-op" || result.Outcome == "readback-verified" {
		if cleanupErr := lifecycle.retainBackups(target.Identifier, 10); cleanupErr != nil {
			result.CleanupWarning = "backup retention failed: " + cleanupErr.Error()
		}
	}
	return result, nil
}
