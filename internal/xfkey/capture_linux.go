package xfkey

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type CaptureResult struct {
	Directory    string         `json:"capture_directory"`
	PhysicalPath string         `json:"physical_path,omitempty"`
	Identity     deviceIdentity `json:"identity"`
	Readback     *Readback      `json:"readback,omitempty"`
	KeyPrefix    string         `json:"key_prefix_hex,omitempty"`
	Warning      string         `json:"warning"`
}

type captureQuery struct {
	result  CaptureResult
	replies map[byte][]byte
	config  []byte
}

type backupRecord struct {
	SchemaVersion int    `json:"schema_version"`
	RecordType    string `json:"record_type"`
	DirectoryName string `json:"directory_name"`
	CreatedUTC    string `json:"created_utc"`
	Model         string `json:"model"`
	Identifier    string `json:"identifier"`
}

var (
	backupNamePattern         = regexp.MustCompile(`^([0-9]{8})-([0-9]{6})-([0-9a-f]{4})$`)
	captureNow                = func() time.Time { return time.Now().UTC() }
	labelledBackupNamePattern = regexp.MustCompile(`^([0-9]{6})-([0-9]{6})_key-[a-z0-9-]+_rgb-[a-z0-9-]+-([0-9a-f]{6}|unknown)(-[2-9]|-[1-9][0-9]+)?$`)
	renameCapture             = unix.Renameat2
	renameBackup              = unix.Renameat2
	unlinkBackup              = unix.Unlinkat
	syncRootFD                = unix.Fsync
)

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func saveExclusive(dir, name string, data []byte) error {
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
	if err := os.Link(filepath.Join(dir, pending), marker); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		removeErr := os.Remove(marker)
		return errors.Join(err, removeErr, syncDir(dir))
	}
	return nil
}

func prepareCaptureRoot(root string, syncDirectory func(string) error) (string, error) {
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("--capture-root must be an absolute directory")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	for path := resolved; ; path = filepath.Dir(path) {
		if err := syncDirectory(path); err != nil {
			return "", fmt.Errorf("synchronise capture ancestor %s: %w", path, err)
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	return resolved, nil
}

func newCapture(root string, target Candidate) (string, error) {
	return newCaptureWithSync(root, target, syncDir)
}

func newCaptureWithSync(root string, target Candidate, syncDirectory func(string) error) (string, error) {
	resolved, err := prepareCaptureRoot(root, syncDirectory)
	if err != nil {
		return "", err
	}
	return newCaptureInRoot(resolved, target, syncDirectory)
}

func newCaptureInRoot(root string, target Candidate, syncDirectory func(string) error) (string, error) {
	started := captureNow().UTC()
	var dir string
	base := started.Format("060102-150405") + "_key-unknown_rgb-unknown-unknown"
	for attempts := 1; attempts <= 100; attempts++ {
		candidate := filepath.Join(root, captureCollisionName(base, attempts))
		if err := os.Mkdir(candidate, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", err
		}
		dir = candidate
		break
	}
	if dir == "" {
		return "", fmt.Errorf("cannot create an exclusive backup directory after 100 attempts")
	}
	if err := syncDirectory(root); err != nil {
		return dir, err
	}
	metadata := struct {
		Timestamp string    `json:"started_utc"`
		Target    Candidate `json:"target"`
		Warning   string    `json:"warning"`
	}{started.Format(time.RFC3339Nano), target, "Query capture only. Not a verified restore image. Missing result.json means incomplete or failed."}
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return dir, err
	}
	return dir, saveExclusive(dir, "provenance.json", b)
}

func captureCollisionName(base string, attempt int) string {
	if attempt == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, attempt)
}

func captureSettingsLabel(raw []byte) string {
	if len(raw) != 128 {
		return "key-unknown_rgb-unknown-unknown"
	}
	key := fmt.Sprintf("unknown-%x", raw[:5])
	if raw[0] == 0 && raw[1] >= 1 && raw[1] <= 3 && raw[3] == 1 && raw[2] <= 15 {
		for name, value := range keyValues {
			if int(raw[4]) == value {
				key = name
				if raw[2] != 0 {
					key = strings.ReplaceAll(modifierName(raw[2]), ",", "-") + "-" + key
				}
			}
		}
	}
	mode := fmt.Sprintf("unknown-%02x", raw[124])
	for name, value := range lightingValues {
		if int(raw[124]) == value+1 {
			mode = name
		}
	}
	return fmt.Sprintf("key-%s_rgb-%s-%x", key, mode, raw[125:128])
}

// Only the newly created, unfinished capture is named here, before its completion record.
func nameCapturedSettings(dir string, raw []byte) (string, error) {
	if len(raw) != 128 {
		return dir, nil
	}
	name := filepath.Base(dir)
	if !labelledBackupNamePattern.MatchString(name) {
		return dir, fmt.Errorf("invalid new capture name %q", name)
	}
	root := filepath.Dir(dir)
	base := name[:13] + "_" + captureSettingsLabel(raw)
	for attempt := 1; attempt <= 100; attempt++ {
		destination := filepath.Join(root, captureCollisionName(base, attempt))
		if err := renameCapture(unix.AT_FDCWD, dir, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return dir, err
		}
		return destination, errors.Join(syncDir(destination), syncDir(root))
	}
	return dir, fmt.Errorf("cannot name an exclusive capture after 100 attempts")
}

func queryCapture(t queryTransport, readback bool) (captureQuery, error) {
	q := captureQuery{result: CaptureResult{Warning: "Current query replies only, not a verified restore image. Field meanings are host-derived."}, replies: map[byte][]byte{}}
	commands := []byte{1}
	if readback {
		commands = append(commands, 6, 7, 8)
	}
	var replies [3][]byte
	for i, command := range commands {
		reply, err := query(t, command)
		if len(reply) > 0 {
			q.replies[command] = append([]byte(nil), reply...)
		}
		if err != nil {
			return q, fmt.Errorf("AF %02x: %w", command, err)
		}
		if i == 0 {
			q.result.Identity, err = parseIdentity(reply)
			if err != nil {
				return q, err
			}
		}
	}
	if readback {
		for i := range replies {
			replies[i] = q.replies[byte(i+6)]
		}
		parsed, err := parseReadback(replies)
		if err != nil {
			return q, err
		}
		q.config, err = hex.DecodeString(parsed.Configuration)
		if err != nil {
			return q, err
		}
		q.result.Readback = &parsed
		q.result.KeyPrefix = hex.EncodeToString(q.config[:5])
	}
	return q, nil
}

func persistCapture(dir string, q captureQuery, complete bool) (CaptureResult, error) {
	q.result.Directory = dir
	commands := []byte{1, 6, 7, 8}
	for _, command := range commands {
		if reply, ok := q.replies[command]; ok {
			if err := saveExclusive(dir, fmt.Sprintf("reply-%02x.bin", command), reply); err != nil {
				return q.result, err
			}
		}
	}
	if !complete {
		return q.result, nil
	}
	if q.config != nil {
		if err := saveExclusive(dir, "configuration.bin", q.config); err != nil {
			return q.result, err
		}
	}
	b, err := json.MarshalIndent(q.result, "", "  ")
	if err != nil {
		return q.result, err
	}
	return q.result, saveCompletion(dir, b)
}

func captureNamedQueries(t queryTransport, dir string, readback bool) (CaptureResult, error) {
	q, queryErr := queryCapture(t, readback)
	var nameErr error
	dir, nameErr = nameCapturedSettings(dir, q.config)
	result, persistErr := persistCapture(dir, q, queryErr == nil && nameErr == nil)
	return result, errors.Join(queryErr, nameErr, persistErr)
}

func captureQueries(t queryTransport, dir string, readback bool) (CaptureResult, error) {
	q, queryErr := queryCapture(t, readback)
	result, persistErr := persistCapture(dir, q, queryErr == nil)
	if queryErr != nil {
		return result, errors.Join(queryErr, persistErr)
	}
	return result, persistErr
}

var (
	liveDiscover   = discover
	liveOpenTarget = func(target Candidate) (queryTransport, io.Closer, error) {
		t, err := openTarget(target)
		return t, t, err
	}
)

func querySelected(target Candidate, readback bool) (CaptureResult, error) {
	t, closer, err := liveOpenTarget(target)
	if err != nil {
		return CaptureResult{}, err
	}
	defer closer.Close()
	q, err := queryCapture(t, readback)
	q.result.PhysicalPath = target.PhysicalPath
	return q.result, err
}

type captureRootLock struct {
	root   string
	rootFD int
	lockFD int
}

func lockCaptureRoot(root string) (*captureRootLock, error) {
	resolved, err := prepareCaptureRoot(root, syncDir)
	if err != nil {
		return nil, err
	}
	rootFD, err := unix.Open(resolved, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	lockFD, err := unix.Openat(rootFD, ".wonkey-lifecycle.lock", unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err == nil {
		err = unix.Flock(lockFD, unix.LOCK_EX)
	}
	if err != nil {
		if lockFD >= 0 {
			unix.Close(lockFD)
		}
		unix.Close(rootFD)
		return nil, err
	}
	return &captureRootLock{resolved, rootFD, lockFD}, nil
}

func (l *captureRootLock) close() error {
	err := unix.Flock(l.lockFD, unix.LOCK_UN)
	return errors.Join(err, unix.Close(l.lockFD), unix.Close(l.rootFD))
}

func parseBackupName(name string) (time.Time, bool) {
	match := labelledBackupNamePattern.FindStringSubmatch(name)
	if match != nil {
		t, err := time.Parse("20060102-150405", "20"+match[1]+"-"+match[2])
		return t.UTC(), err == nil
	}
	match = backupNamePattern.FindStringSubmatch(name)
	if match == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102-150405", match[1]+"-"+match[2])
	return t.UTC(), err == nil
}

func saveBackupRecord(dir string, identity deviceIdentity) error {
	name := filepath.Base(dir)
	created, ok := parseBackupName(name)
	if !ok {
		return fmt.Errorf("invalid apply backup directory name %q", name)
	}
	record := backupRecord{1, "wonkey-apply-backup", name, created.Format(time.RFC3339), fmt.Sprintf("%04x", identity.Model), identity.Identifier}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return saveExclusive(dir, "backup.json", b)
}

func decodeStrict(data []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

type fileIdentity struct {
	device uint64
	inode  uint64
}

type ownedBackup struct {
	name     string
	created  time.Time
	identity fileIdentity
}

var allowedBackupFiles = map[string]bool{
	"provenance.json": true, "backup.json": true, ".result.json.pending": true, "result.json": true,
	"reply-01.bin": true, "reply-06.bin": true, "reply-07.bin": true, "reply-08.bin": true,
	"configuration.bin": true, "plan.json": true, "intended-configuration.bin": true, "apply-outcome.json": true,
	"write-1-request.bin": true, "write-1-reply.bin": true, "write-2-request.bin": true, "write-2-reply.bin": true,
	"write-3-request.bin": true, "write-3-reply.bin": true, "write-4-request.bin": true, "write-4-reply.bin": true,
	"post-reply-06.bin": true, "post-reply-07.bin": true, "post-reply-08.bin": true, "post-configuration.bin": true,
}

type outcomeChangeOwnership struct {
	Setting *string `json:"setting"`
	Before  *string `json:"before"`
	After   *string `json:"after"`
}

type outcomeOwnership struct {
	Directory           *string                   `json:"capture_directory"`
	Outcome             *string                   `json:"outcome"`
	Changes             *[]outcomeChangeOwnership `json:"changes"`
	CommitEcho          *bool                     `json:"commit_echo_received"`
	ReadbackVerified    *bool                     `json:"all_128_bytes_readback_verified"`
	WriteAttempted      *bool                     `json:"write_attempted"`
	PersistenceVerified *bool                     `json:"persistence_after_reconnect_verified"`
	Error               *string                   `json:"error,omitempty"`
	Warning             *string                   `json:"warning"`
	CleanupWarning      *string                   `json:"cleanup_warning,omitempty"`
}

func readCaptureAt(dirFD int, name string, limit int64) ([]byte, error) {
	fd, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Size > limit {
		return nil, fmt.Errorf("not a bounded regular capture file: %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("capture file exceeds size limit: %s", name)
	}
	return data, nil
}

func directoryEntries(dirFD int) (map[string]fileIdentity, error) {
	dup, err := unix.Dup(dirFD)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(dup), "capture-directory")
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]fileIdentity, len(names))
	for _, name := range names {
		var stat unix.Stat_t
		if !allowedBackupFiles[name] || unix.Fstatat(dirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return nil, fmt.Errorf("unexpected backup entry: %s", name)
		}
		entries[name] = fileIdentity{stat.Dev, stat.Ino}
	}
	return entries, nil
}

func loadCaptureAt(dirFD int) (CaptureResult, configuration, error) {
	var result CaptureResult
	var config configuration
	data, err := readCaptureAt(dirFD, "result.json", 65536)
	if err != nil || decodeStrict(data, &result) != nil {
		return result, config, errors.Join(err, fmt.Errorf("invalid result.json"))
	}
	identityRaw, err := readCaptureAt(dirFD, "reply-01.bin", 64)
	if err != nil {
		return result, config, err
	}
	identity, err := parseIdentity(identityRaw)
	if err != nil || result.Identity != identity {
		return result, config, fmt.Errorf("capture identity does not match raw reply")
	}
	var replies [3][]byte
	for i := range replies {
		replies[i], err = readCaptureAt(dirFD, fmt.Sprintf("reply-%02x.bin", i+6), 64)
		if err != nil {
			return result, config, err
		}
	}
	parsed, err := parseReadback(replies)
	if err != nil || result.Readback == nil || *result.Readback != parsed {
		return result, config, fmt.Errorf("capture readback does not match raw replies")
	}
	data, err = readCaptureAt(dirFD, "configuration.bin", 128)
	if err != nil || len(data) != 128 || hex.EncodeToString(data) != parsed.Configuration {
		return result, config, fmt.Errorf("capture configuration does not match raw replies")
	}
	copy(config[:], data)
	return result, config, nil
}

func validOutcomeChange(change outcomeChangeOwnership) bool {
	return change.Setting != nil && change.Before != nil && change.After != nil && *change.Setting != "" && *change.Before != "" && *change.After != "" && *change.Before != *change.After
}

func validateOutcome(data []byte, dir string) bool {
	var outcome outcomeOwnership
	if decodeStrict(data, &outcome) != nil || outcome.Directory == nil || outcome.Outcome == nil || outcome.Changes == nil || outcome.CommitEcho == nil || outcome.ReadbackVerified == nil || outcome.WriteAttempted == nil || outcome.PersistenceVerified == nil || outcome.Warning == nil {
		return false
	}
	if *outcome.Directory != dir || *outcome.Warning == "" || *outcome.PersistenceVerified || outcome.Error != nil && *outcome.Error != "" || outcome.CleanupWarning != nil && *outcome.CleanupWarning != "" {
		return false
	}
	for _, change := range *outcome.Changes {
		if !validOutcomeChange(change) {
			return false
		}
	}
	switch *outcome.Outcome {
	case "no-op":
		return len(*outcome.Changes) == 0 && !*outcome.CommitEcho && !*outcome.ReadbackVerified && !*outcome.WriteAttempted
	case "readback-verified":
		return len(*outcome.Changes) > 0 && *outcome.CommitEcho && *outcome.ReadbackVerified && *outcome.WriteAttempted
	default:
		return false
	}
}

func classifyOwnedBackupAt(rootFD int, root, name, model, identifier string) (ownedBackup, bool) {
	_, ok := parseBackupName(name)
	if !ok {
		return ownedBackup{}, false
	}
	dirFD, err := unix.Openat(rootFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return ownedBackup{}, false
	}
	defer unix.Close(dirFD)
	backup, _, ok := classifyOwnedBackupFD(dirFD, root, name, model, identifier)
	return backup, ok
}

func classifyOwnedBackup(root, name, model, identifier string) (ownedBackup, bool) {
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return ownedBackup{}, false
	}
	defer unix.Close(rootFD)
	return classifyOwnedBackupAt(rootFD, root, name, model, identifier)
}

func (l *captureRootLock) retainBackups(identifier string, keep int) error {
	const model = "0112"
	entries, err := directoryEntriesForRoot(l.rootFD)
	if err != nil {
		return err
	}
	var backups []ownedBackup
	for _, name := range entries {
		if backup, ok := classifyOwnedBackupAt(l.rootFD, l.root, name, model, identifier); ok {
			backups = append(backups, backup)
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].created.Equal(backups[j].created) {
			left := labelledBackupNamePattern.FindStringSubmatch(backups[i].name)
			right := labelledBackupNamePattern.FindStringSubmatch(backups[j].name)
			if left != nil && right != nil {
				leftBase := strings.TrimSuffix(backups[i].name, left[4])
				rightBase := strings.TrimSuffix(backups[j].name, right[4])
				if leftBase == rightBase {
					leftIndex, _ := strconv.Atoi(strings.TrimPrefix(left[4], "-"))
					rightIndex, _ := strconv.Atoi(strings.TrimPrefix(right[4], "-"))
					return leftIndex > rightIndex
				}
			}
			return backups[i].name > backups[j].name
		}
		return backups[i].created.After(backups[j].created)
	})
	if len(backups) <= keep {
		return nil
	}
	var cleanupErr error
	for _, backup := range backups[keep:] {
		if err := l.removeBackup(backup, model, identifier); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove backup %s: %w", backup.name, err))
		}
	}
	return cleanupErr
}

func directoryEntriesForRoot(rootFD int) ([]string, error) {
	fd, err := unix.Openat(rootFD, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "capture-root")
	defer f.Close()
	return f.Readdirnames(-1)
}

func (l *captureRootLock) removeBackup(backup ownedBackup, model, identifier string) error {
	tombstone := ".wonkey-remove-" + backup.name
	if err := renameBackup(l.rootFD, backup.name, l.rootFD, tombstone, unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	if err := syncRootFD(l.rootFD); err != nil {
		return err
	}
	dirFD, err := unix.Openat(l.rootFD, tombstone, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer unix.Close(dirFD)
	var stat unix.Stat_t
	if err := unix.Fstat(dirFD, &stat); err != nil || backup.identity != (fileIdentity{stat.Dev, stat.Ino}) {
		return fmt.Errorf("backup directory identity changed")
	}
	validated, entries, ok := classifyOwnedBackupFD(dirFD, l.root, backup.name, model, identifier)
	if !ok || validated.identity != backup.identity {
		return fmt.Errorf("backup changed after retention classification")
	}
	for child, identity := range entries {
		if err := unix.Fstatat(dirFD, child, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil || identity != (fileIdentity{stat.Dev, stat.Ino}) || stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("backup entry changed before deletion: %s", child)
		}
		if err := unlinkBackup(dirFD, child, 0); err != nil {
			return err
		}
	}
	if err := unlinkBackup(l.rootFD, tombstone, unix.AT_REMOVEDIR); err != nil {
		return err
	}
	return syncRootFD(l.rootFD)
}

func classifyOwnedBackupFD(dirFD int, root, name, model, identifier string) (ownedBackup, map[string]fileIdentity, bool) {
	created, ok := parseBackupName(name)
	if !ok {
		return ownedBackup{}, nil, false
	}
	var dirStat unix.Stat_t
	if unix.Fstat(dirFD, &dirStat) != nil {
		return ownedBackup{}, nil, false
	}
	entries, err := directoryEntries(dirFD)
	if err != nil {
		return ownedBackup{}, nil, false
	}
	marker, err := readCaptureAt(dirFD, "backup.json", 4096)
	if err != nil {
		return ownedBackup{}, nil, false
	}
	var record backupRecord
	if decodeStrict(marker, &record) != nil || record.SchemaVersion != 1 || record.RecordType != "wonkey-apply-backup" || record.DirectoryName != name || record.CreatedUTC != created.Format(time.RFC3339) || record.Model != model || record.Identifier != identifier {
		return ownedBackup{}, nil, false
	}
	capture, _, err := loadCaptureAt(dirFD)
	dir := filepath.Join(root, name)
	if err != nil || fmt.Sprintf("%04x", capture.Identity.Model) != record.Model || capture.Identity.Identifier != record.Identifier || capture.Directory != dir {
		return ownedBackup{}, nil, false
	}
	outcomeRaw, err := readCaptureAt(dirFD, "apply-outcome.json", 65536)
	if err != nil || !validateOutcome(outcomeRaw, dir) {
		return ownedBackup{}, nil, false
	}
	return ownedBackup{name, created, fileIdentity{dirStat.Dev, dirStat.Ino}}, entries, true
}
