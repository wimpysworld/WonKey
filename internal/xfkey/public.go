package xfkey

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const publicHelp = `WonKey  One key. Your rules.
Configure an XFKEY One Key Max (0112).

Usage: wonkey <command> [options]

Commands:
  key [COMBINATION]   Read or change the key.
  rgb [MODE [RGB]]    Read or change lighting.
  restore [DIRECTORY] Restore saved key and lighting settings.

Options:
  -h, --help          Show help without device access.

Examples:
  wonkey key
  wonkey key f13
  wonkey key ctrl+shift+f13 --on release
  wonkey rgb steady 0000ff
  wonkey rgb off

Run "wonkey <command> --help" for values and options.
`

const keyHelp = `Usage: wonkey key [COMBINATION] [--on WHEN]

Without a combination, read the current key. Create no files.
A combination contains one key: a-z, 0-9, enter or f1 through f24.
Optional modifiers precede the key: ctrl+, shift+, alt+ or super+, each at most once.
Key names ignore case. Uppercase letters do not add Shift.
Unspecified modifiers are cleared. Omitted --on preserves the trigger.

Options:
  --on WHEN     press, release or both. Requires a key combination.
  -h, --help    Show help without device access.

Examples:
  wonkey key
  wonkey key f13
  wonkey key ctrl+shift+f13
  wonkey key f13 --on release

Changes require a terminal. At [Y/n], Enter accepts. No or EOF cancels.
WonKey saves and validates a fresh backup before writing.
`

const rgbHelp = `Usage: wonkey rgb [MODE [RGB]]

Without a mode, read current lighting. Create no files.
RGB is exactly six hexadecimal digits, for example 0000ff for blue.
Omit RGB to preserve the current colour. Key settings stay unchanged.

Modes:
  gradient    Full-colour gradient
  steady      Single-colour steady
  flowing     Single-colour flowing
  flash       Flash on click
  neon        Neon flowing
  off         Lights off
  held        On while pressed, off on release
  toggle      Toggle on click

Options:
  -h, --help   Show help without device access.

Examples:
  wonkey rgb
  wonkey rgb steady 0000ff
  wonkey rgb steady
  wonkey rgb off

Changes require a terminal. At [Y/n], Enter accepts. No or EOF cancels.
WonKey saves and validates a fresh backup before writing.
Mode descriptions are vendor labels, not verified lighting effects.
`

type publicCommand struct {
	name    string
	source  string
	changes Changes
	help    bool
}

func parsePublic(args []string) (publicCommand, error) {
	c := publicCommand{}
	if len(args) == 0 {
		c.help = true
		return c, nil
	}
	if args[0] == "--help" || args[0] == "-h" {
		if len(args) != 1 {
			return c, fmt.Errorf("help accepts no other arguments")
		}
		c.help = true
		return c, nil
	}
	c.name = args[0]
	if c.name != "key" && c.name != "rgb" && c.name != "restore" {
		return c, fmt.Errorf("unknown command %q; use key, rgb or restore", c.name)
	}
	var positional []string
	on, seenOn := "", false
	args = args[1:]
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		switch {
		case arg == "--help" || arg == "-h":
			c.help = true
		case arg == "--on" || strings.HasPrefix(arg, "--on="):
			if c.name != "key" || seenOn {
				return c, fmt.Errorf("--on is accepted once, for key only")
			}
			seenOn = true
			if arg == "--on" {
				if len(args) == 0 {
					return c, fmt.Errorf("--on needs press, release or both")
				}
				on = args[0]
				args = args[1:]
			} else {
				on = strings.TrimPrefix(arg, "--on=")
			}
			if _, ok := triggerValues[strings.ToLower(on)]; !ok {
				return c, fmt.Errorf("--on needs press, release or both")
			}
		case strings.HasPrefix(arg, "-"):
			return c, fmt.Errorf("unknown option %q", arg)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		if seenOn {
			return c, fmt.Errorf("--on requires a key combination")
		}
		return c, nil
	}
	return c.parseValues(positional, on)
}

func (c publicCommand) parseValues(positional []string, on string) (publicCommand, error) {
	if c.name == "restore" {
		if len(positional) != 1 {
			return c, fmt.Errorf("restore accepts one optional capture directory")
		}
		if positional[0] == "" {
			return c, fmt.Errorf("restore source directory must not be empty")
		}
		c.source = positional[0]
		return c, nil
	}
	var err error
	if c.name == "key" {
		if len(positional) != 1 {
			return c, fmt.Errorf("key accepts one complete combination")
		}
		parts := strings.Split(strings.ToLower(positional[0]), "+")
		if _, ok := keyValues[parts[len(parts)-1]]; !ok {
			return c, fmt.Errorf("supported keys are a-z, 0-9, enter and f1 through f24")
		}
		modifiers := "none"
		if len(parts) > 1 {
			for _, p := range parts[:len(parts)-1] {
				if p != "ctrl" && p != "shift" && p != "alt" && p != "super" {
					return c, fmt.Errorf("invalid modifier %q", p)
				}
			}
			modifiers = strings.Join(parts[:len(parts)-1], ",")
		}
		c.changes, err = parseSettings(parts[len(parts)-1], on, modifiers, "", "")
	} else {
		if len(positional) > 2 {
			return c, fmt.Errorf("rgb accepts a mode and optional six-digit colour")
		}
		if _, ok := lightingValues[strings.ToLower(positional[0])]; !ok {
			return c, fmt.Errorf("unknown RGB mode %q", positional[0])
		}
		colour := ""
		if len(positional) == 2 {
			colour = positional[1]
			if len(colour) != 6 {
				return c, fmt.Errorf("RGB needs exactly six hexadecimal digits")
			}
		}
		c.changes, err = parseSettings("", "", "", positional[0], colour)
	}
	if err != nil {
		return c, fmt.Errorf("invalid %s value: %s", c.name, strings.ReplaceAll(err.Error(), "--", ""))
	}
	return c, nil
}

type publicAccess struct {
	discover    func() ([]Candidate, error)
	query       func(Candidate) (CaptureResult, error)
	apply       func(Candidate, string, ApplyTarget, Changes, func(configuration, configuration, string) (bool, error)) (ApplyResult, error)
	interactive func(io.Reader) bool
}

func publicDeviceAccess() publicAccess {
	return publicAccess{
		discover: func() ([]Candidate, error) { return liveDiscover("/sys/bus/usb/devices", "/dev") },
		query:    func(selected Candidate) (CaptureResult, error) { return querySelected(selected, true) },
		apply: func(selected Candidate, root string, target ApplyTarget, changes Changes, guard func(configuration, configuration, string) (bool, error)) (ApplyResult, error) {
			return liveApplyBound(selected.PhysicalPath, root, target, changes, true, guard, &selected, true)
		},
		interactive: readerInteractive,
	}
}

func Run(args []string, stdout, stderr io.Writer) error {
	return RunIO(args, strings.NewReader(""), stdout, stderr)
}

func RunIO(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runPublic(args, &cliRuntime{stdin, stdout, stderr}, publicDeviceAccess())
}

func runPublic(args []string, rt *cliRuntime, access publicAccess) error {
	c, err := parsePublic(args)
	code := 2
	if err == nil {
		code = 1
		if c.help {
			text := publicHelp
			if c.name == "key" {
				text = keyHelp
			}
			if c.name == "rgb" {
				text = rgbHelp
			}
			if c.name == "restore" {
				text = restoreHelp
			}
			return printHumanHelp(rt.out, text)
		}
		err = c.run(rt, access)
	}
	if err != nil {
		newHuman(rt.errOut).line("31", "Error: "+err.Error()+".")
		fmt.Fprintln(rt.errOut, `Run "wonkey --help" for commands.`)
		return &CLIError{Err: err, Code: code}
	}
	return nil
}

func choosePublic(candidates []Candidate, input *bufio.Reader, h human, interactive bool) (*Candidate, error) {
	var matches []Candidate
	for _, c := range candidates {
		if c.Compatible {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no compatible device found")
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if !interactive {
		return nil, fmt.Errorf("multiple devices require terminal selection; no HID device opened")
	}
	h.line("36", "Select a device for this command:")
	for i, c := range matches {
		h.field(strconv.Itoa(i+1), c.PhysicalPath+" ("+c.VendorNode.Path+")")
	}
	fmt.Fprint(h.out, "Device number [cancel]: ")
	if h.out.err != nil {
		return nil, h.out.err
	}
	line, err := input.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
	if err != nil || n < 1 || n > len(matches) {
		return nil, nil
	}
	return &matches[n-1], nil
}

func (c publicCommand) run(rt *cliRuntime, access publicAccess) error {
	interactive := access.interactive(rt.in)
	if (len(c.changes) > 0 || c.name == "restore") && !interactive {
		return fmt.Errorf("changes require an interactive terminal; no device access")
	}
	input := bufio.NewReader(rt.in)
	h := newHuman(rt.out)
	candidates, err := access.discover()
	if err != nil {
		return err
	}
	selected, err := choosePublic(candidates, input, h, interactive)
	if err != nil {
		return err
	}
	if selected == nil {
		h.line("", "Cancelled. No HID device opened.")
		return h.out.err
	}
	observed, err := access.query(*selected)
	if err != nil {
		return err
	}
	target := ApplyTarget{observed.Identity.Identifier, fmt.Sprintf("%04x", observed.Identity.Version)}
	if err := target.validate(); err != nil {
		return err
	}
	if !target.matches(observed.Identity) || observed.PhysicalPath != selected.PhysicalPath {
		return fmt.Errorf("unsupported or changed device identity/path")
	}
	if observed.Readback == nil {
		return fmt.Errorf("device settings are missing")
	}
	return c.runObserved(observed, *selected, target, input, h, access)
}

func (c publicCommand) showCurrentSettings(current configuration, selected Candidate, target ApplyTarget, h human) {
	heading := "Current " + c.name
	if c.name == "restore" {
		heading = "Current settings"
	}
	h.heading(heading)
	h.field("Device", "One Key Max 0112")
	h.field("USB path", selected.PhysicalPath)
	h.field("Identifier", target.Identifier)
	h.field("Version", target.Version)
	if c.name != "rgb" {
		key := settingName(keyValues, int(current[4]))
		if current[2] != 0 {
			key = strings.ReplaceAll(modifierName(current[2]), ",", "+") + "+" + key
		}
		h.field("Key", key)
		h.field("On", settingName(triggerValues, int(current[1])))
	}
	if c.name != "key" {
		h.field("Mode", settingName(lightingValues, int(current[124])-1))
		fmt.Fprintf(h.out, "  %-10s %s\n", "Colour", h.rgb(fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127])))
	}
}

func (c publicCommand) runObserved(observed CaptureResult, selected Candidate, target ApplyTarget, input *bufio.Reader, h human, access publicAccess) error {
	raw, err := hex.DecodeString(observed.Readback.Configuration)
	if err != nil {
		return err
	}
	var current configuration
	if len(raw) != len(current) {
		return fmt.Errorf("device settings need exactly 128 bytes")
	}
	copy(current[:], raw)
	if err := supportedConfiguration(current); err != nil {
		return err
	}
	c.showCurrentSettings(current, selected, target, h)
	if h.out.err != nil {
		return h.out.err
	}
	if c.name == "restore" {
		source, err := chooseRestore(c.source, observed.Identity, current, input, h)
		if err != nil {
			return err
		}
		if source == nil {
			return nil
		}
		if c.source != "" {
			h.field("Source", source.directory)
		}
		h.line("", "Restore saved key and lighting settings. Keep all other current bytes.")
		c.changes = restoreChanges(source.config)
		restored, err := changeConfiguration(current, c.changes)
		if err != nil {
			return err
		}
		if restored != source.config {
			h.line("33", "Other saved bytes differ. Those current bytes will remain unchanged.")
		}
	}
	if len(c.changes) == 0 {
		return h.out.err
	}
	intended, err := changeConfiguration(current, c.changes)
	if err != nil {
		return err
	}
	views := settingViews(current, intended)
	h.changes(views)
	if len(views) == 0 {
		h.line("", "No settings write sent.")
		return h.out.err
	}
	fmt.Fprint(h.out, "Save settings? [Y/n]: ")
	if h.out.err != nil {
		return h.out.err
	}
	line, err := input.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if errors.Is(err, io.EOF) || !saveConfirmation(line) {
		h.line("", "Cancelled. No settings write sent.")
		return h.out.err
	}
	root, err := resolveCaptureRoot()
	if err != nil {
		return err
	}
	guard := func(fresh, proposed configuration, _ string) (bool, error) {
		if fresh != current || proposed != intended {
			return false, fmt.Errorf("settings changed after preview; no settings write sent")
		}
		return true, nil
	}
	result, err := access.apply(selected, root, target, c.changes, guard)
	if err != nil {
		if result.Directory != "" {
			h.field("Records", result.Directory)
		}
		if result.WriteAttempted {
			h.line("31", "Device state uncertain. A submitted write can still complete.")
		} else {
			h.line("", "Stopped before settings upload.")
		}
		h.line("33", "Stop. No automatic retry or rollback.")
		return err
	}
	if result.ReadbackVerified {
		h.line("32", "Settings saved. All 128 readback bytes match.")
	} else {
		h.line("", "No settings write sent.")
	}
	if result.CleanupWarning != "" {
		h.line("33", "Warning: "+result.CleanupWarning+". Do not retry a successful write.")
	}
	return nil
}
