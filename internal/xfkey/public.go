package xfkey

import (
	"bufio"
	"encoding/hex"
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

Options:
  -h, --help          Show help without device access.

Examples:
  wonkey key
  wonkey key f13
  wonkey key ctrl+shift+f13 --on release
  wonkey rgb steady 0000ff
  wonkey rgb off

Run "wonkey key --help" or "wonkey rgb --help" for values and options.
`

const keyHelp = `Usage: wonkey key [COMBINATION] [--on WHEN]

Without a combination, read the current key. Create no files.
A combination is the complete key: enter or f13, optionally preceded by
ctrl+, shift+, alt+ or gui+ (Super/Windows/Command), each at most once.
Unspecified modifiers are cleared. Omitted --on preserves the trigger.

Options:
  --on WHEN     press, release or both. Requires a key combination.
  -h, --help    Show help without device access.

Examples:
  wonkey key
  wonkey key f13
  wonkey key ctrl+shift+f13
  wonkey key f13 --on release

Changes require a terminal and the exact answer write. Blank input cancels.
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

Changes require a terminal and the exact answer write. Blank input cancels.
WonKey saves and validates a fresh backup before writing.
Mode descriptions are vendor labels, not verified lighting effects.
`

type publicCommand struct {
	name    string
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
	if c.name != "key" && c.name != "rgb" {
		return c, fmt.Errorf("unknown command %q; use key or rgb", c.name)
	}
	var positional []string
	on, seenOn := "", false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help" || arg == "-h":
			c.help = true
		case arg == "--on" || strings.HasPrefix(arg, "--on="):
			if c.name != "key" || seenOn {
				return c, fmt.Errorf("--on is accepted once, for key only")
			}
			seenOn = true
			if arg == "--on" {
				i++
				if i == len(args) {
					return c, fmt.Errorf("--on needs press, release or both")
				}
				on = args[i]
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
	var err error
	if c.name == "key" {
		if len(positional) != 1 {
			return c, fmt.Errorf("key accepts one complete combination")
		}
		parts := strings.Split(strings.ToLower(positional[0]), "+")
		if _, ok := keyValues[parts[len(parts)-1]]; !ok {
			return c, fmt.Errorf("supported keys are enter and f13")
		}
		modifiers := "none"
		if len(parts) > 1 {
			for _, p := range parts[:len(parts)-1] {
				if p != "ctrl" && p != "shift" && p != "alt" && p != "gui" {
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
	if len(c.changes) == 0 {
		return c, fmt.Errorf("empty setting")
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
	line, err := input.ReadString('\n')
	if err != nil {
		if err == io.EOF {
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
	if len(c.changes) > 0 && !interactive {
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
		return nil
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
	h.heading("Current " + c.name)
	h.field("Device", "One Key Max 0112")
	h.field("USB path", selected.PhysicalPath)
	h.field("Identifier", target.Identifier)
	h.field("Version", target.Version)
	if c.name == "key" {
		key := map[byte]string{0x28: "enter", 0x68: "f13"}[current[4]]
		if current[2] != 0 {
			key = strings.ReplaceAll(modifierName(current[2]), ",", "+") + "+" + key
		}
		h.field("Key", key)
		h.field("On", map[byte]string{1: "press", 2: "release", 3: "both"}[current[1]])
	} else {
		h.field("Mode", []string{"", "gradient", "steady", "flowing", "flash", "neon", "off", "held", "toggle"}[current[124]])
		fmt.Fprintf(h.out, "  %-10s %s\n", "Colour", h.rgb(fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127])))
	}
	if len(c.changes) == 0 {
		return nil
	}
	intended, err := changeConfiguration(current, c.changes)
	if err != nil {
		return err
	}
	views := settingViews(current, intended)
	h.changes(views)
	if len(views) == 0 {
		h.line("", "No settings write sent.")
		return nil
	}
	fmt.Fprint(h.out, "Save settings? [Y/n]: ")
	line, err := input.ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	answer := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
	if err == io.EOF || (answer != "" && answer != "y" && answer != "yes") {
		h.line("", "Cancelled. No settings write sent.")
		return nil
	}
	root, err := resolveCaptureRoot("")
	if err != nil {
		return err
	}
	guard := func(fresh, proposed configuration, _ string) (bool, error) {
		if fresh != current || proposed != intended {
			return false, fmt.Errorf("settings changed after preview; no settings write sent")
		}
		return true, nil
	}
	result, err := access.apply(*selected, root, target, c.changes, guard)
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
