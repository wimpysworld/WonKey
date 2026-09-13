package xfkey

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const restoreHelp = `Usage: wonkey restore [CAPTURE-DIRECTORY]

Restore saved key, modifiers, trigger, RGB mode and colour only.
Keep every other current byte. This is not a firmware or full-image restore.
Without a directory, select a compatible capture from automatic storage.
Captures with matching supported settings are excluded from the list.
An explicit directory can be outside automatic storage.
New backups always use XDG_STATE_HOME/wonkey/captures, or
HOME/.local/state/wonkey/captures when XDG_STATE_HOME is not absolute.

Options:
  -h, --help   Show help without device access.

Changes require a terminal. At [Y/n], Enter accepts. No or EOF cancels.
WonKey saves and validates a fresh backup before writing.
Readback does not prove hardware effects or persistence after reconnect.
`

type restoreSource struct {
	directory string
	capture   CaptureResult
	config    configuration
	created   time.Time
}

func restoreChanges(saved configuration) Changes {
	changes := make(Changes, len(settingsFields))
	for name, spec := range settingsFields {
		changes[name] = int(saved[spec.offset]) - spec.shift
	}
	return changes
}

func loadRestoreSourceAt(parentFD int, path, directory string) (restoreSource, error) {
	source := restoreSource{directory: directory}
	fd, err := unix.Openat(parentFD, path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return source, err
	}
	defer unix.Close(fd)
	source.capture, source.config, err = loadCaptureAt(fd)
	if err != nil {
		return source, err
	}
	if err := supportedConfiguration(source.config); err != nil {
		return source, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return source, err
	}
	source.created, _ = parseBackupName(filepath.Base(directory))
	if source.created.IsZero() {
		source.created = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec).UTC()
	}
	return source, nil
}

func loadRestoreSource(path string) (restoreSource, error) {
	directory, err := filepath.Abs(path)
	if err != nil {
		return restoreSource{}, err
	}
	return loadRestoreSourceAt(unix.AT_FDCWD, directory, directory)
}

func (source restoreSource) compatible(identity deviceIdentity) bool {
	return source.capture.Identity.Model == identity.Model &&
		source.capture.Identity.Version == identity.Version &&
		source.capture.Identity.Identifier == identity.Identifier
}

func listRestoreSources(root string, identity deviceIdentity, current configuration) ([]restoreSource, error) {
	root, err := filepath.EvalSymlinks(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	names, err := directoryEntriesForRoot(fd)
	if err != nil {
		return nil, err
	}
	var sources []restoreSource
	for _, name := range names {
		source, err := loadRestoreSourceAt(fd, name, filepath.Join(root, name))
		if err != nil || !source.compatible(identity) {
			continue
		}
		intended, err := changeConfiguration(current, restoreChanges(source.config))
		if err == nil && intended != current {
			sources = append(sources, source)
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].created.Equal(sources[j].created) {
			return sources[i].directory > sources[j].directory
		}
		return sources[i].created.After(sources[j].created)
	})
	return sources, nil
}

func chooseRestore(path string, identity deviceIdentity, current configuration, input *bufio.Reader, h human) (*restoreSource, error) {
	if path != "" {
		source, err := loadRestoreSource(path)
		if err != nil {
			return nil, fmt.Errorf("invalid restore source: %w", err)
		}
		if !source.compatible(identity) {
			return nil, fmt.Errorf("restore source model, version or identifier differs from the selected device")
		}
		return &source, nil
	}
	root, err := resolveCaptureRoot()
	if err != nil {
		return nil, err
	}
	sources, err := listRestoreSources(root, identity, current)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		h.line("", "No compatible captures with different supported settings. No settings write sent.")
		return nil, h.out.err
	}
	h.line("36", "Restore a backup:")
	for i, source := range sources {
		key := settingName(keyValues, int(source.config[4]))
		if source.config[2] != 0 {
			key = strings.ReplaceAll(modifierName(source.config[2]), ",", "+") + "+" + key
		}
		trigger := map[byte]string{1: "press", 2: "release", 3: "both"}[source.config[1]]
		mode := []string{"", "gradient", "steady", "flowing", "flash", "neon", "off", "held", "toggle"}[source.config[124]]
		h.line("", fmt.Sprintf("  %d  %s (%s) | %s #%02X%02X%02X | %s", i+1, key, trigger, mode,
			source.config[125], source.config[126], source.config[127], source.created.Local().Format("02 Jan 2006 15:04")))
	}
	fmt.Fprint(h.out, "Backup number [cancel]: ")
	if h.out.err != nil {
		return nil, h.out.err
	}
	line, err := input.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	n, parseErr := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
	if errors.Is(err, io.EOF) || parseErr != nil || n < 1 || n > len(sources) {
		h.line("", "Cancelled. No settings write sent.")
		return nil, h.out.err
	}
	return &sources[n-1], nil
}
