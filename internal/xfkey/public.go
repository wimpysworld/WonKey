package xfkey

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"wonkey/internal/ch57"
)

const publicHelp = `Inspect and configure supported macropads.

Usage: wonkey [command] [options]

Without a command, inspect all connected supported devices.

Commands:
  key <COMBINATION>   Change the key.
  mouse <BUTTONS>     Change the mouse action.
  media <ACTION>      Change the media action.
  multi <KEY,...>     Change the key sequence.
  rgb <MODE> [RGB]    Change the RGB lighting.

Options:
  -h, --help          Show help without device access.

Examples:
  wonkey
` + `  wonkey key f13
  wonkey key ctrl+shift+f13 --on release
  wonkey rgb static 0000ff
  wonkey rgb off

Run "wonkey <command> --help" for values and options.
`

const keyHelp = `Usage: wonkey key <COMBINATION> [--on WHEN]

Run "wonkey" without arguments to inspect connected devices.
A combination contains exactly one base key from the names below.
Optional modifiers precede the key: ctrl+, shift+, alt+ or super+, each at most once.
Key names ignore case. Uppercase letters do not add Shift.
Unspecified modifiers are cleared. Omitted --on preserves a keyboard trigger.
When replacing another action, the default trigger is press.

Base keys:
  a-z, 0-9, f1-f24, enter, esc, backspace, tab, space
  minus, equal, leftbracket, rightbracket, backslash, nonushash
  semicolon, apostrophe, grave, comma, period, slash, capslock
  printscreen, scrolllock, pause, insert, home, pageup, delete, end, pagedown
  right, left, down, up, numlock
  keypad0-keypad9, keypaddivide, keypadmultiply, keypadminus, keypadplus
  keypadenter, keypadperiod, keypadequal, keypadcomma, keypadequalas400
  nonusbackslash, application, power, execute, help, menu, select, stop
  again, undo, cut, copy, paste, find, mute, volumeup, volumedown
  lockingcapslock, lockingnumlock, lockingscrolllock
  international1-international9, lang1, lang2

Aliases:
  escape=esc, pgup=pageup, pgdn=pagedown
  arrowright=right, arrowleft=left, arrowdown=down, arrowup=up

Names select HID keyboard usages, not text or consumer media commands.
Raw usage numbers, punctuation symbols and right modifiers are unsupported.
Added keys fit the captured descriptor but remain hardware-unverified.

Options:
  --on WHEN     press, release or both. Requires a key combination.
  -h, --help    Show help without device access.

Examples:
  wonkey key f13
  wonkey key ctrl+shift+f13
  wonkey key f13 --on release

At [Y/n], Enter accepts. No or EOF cancels.
`

const rgbHelp = `Usage: wonkey rgb <MODE> [RGB]

Run "wonkey" without arguments to inspect connected devices.
RGB is six hexadecimal digits without #, for example 0000ff for blue.
Omit RGB to preserve the current colour. Key settings stay unchanged.

Modes:
  static [RGB]   Single static colour
  breathe [RGB]  Single breathing colour
  cycle-slow     Full-colour slow cycle
  cycle-fast     Full-colour fast cycle
  flash [RGB]    Flash on click
  held [RGB]     On while pressed, off on release
  toggle [RGB]   Toggle on click
  off            Lights off

Options:
  -h, --help   Show help without device access.

Examples:
  wonkey rgb static 0000ff
  wonkey rgb static
  wonkey rgb off

At [Y/n], Enter accepts. No or EOF cancels.
`

const mouseHelp = `Usage: wonkey mouse <BUTTONS> [--x N] [--y N] [--wheel N]

Run "wonkey" without arguments to inspect connected devices.
Buttons: left, right, middle, none. Join buttons with +, for example left+right.
Use none alone for movement without a button, or a zero action.
Use wheelup or wheeldown alone for one wheel tick. Do not combine with --wheel.
X, Y and wheel accept -127..127. Omitted values are zero.

At [Y/n], Enter accepts. No or EOF cancels.
`

const mediaHelp = `Usage: wonkey media <ACTION|0xHHHH>

Run "wonkey" without arguments to inspect connected devices.
Names ignore case:
  play, pause, record, fastforward, rewind, next, prev, previous, stop, eject
  playpause, mute, volumeup, volumedown, calculator, mycomputer, browser, email
  search, home, back, forward, refresh, bookmarks
Raw usages need exactly four hexadecimal digits and must be 0x0001..0x023c.

At [Y/n], Enter accepts. No or EOF cancels.
`

const multiHelp = `Usage: wonkey multi <KEY[,KEY...]> [--interval MS] [--repeat COUNT]

Run "wonkey" without arguments to inspect connected devices.
Use 1..115 comma-separated base key names from "wonkey key --help".
Modifiers and raw usage numbers are unsupported.
--interval accepts 1..65535 milliseconds (default: 50).
--repeat accepts 1..255 repetitions (default: 1).

At [Y/n], Enter accepts. No or EOF cancels.
`

type publicCommand struct {
	name    string
	changes Changes
	action  Action
	help    bool
}

func parsePublic(args []string) (publicCommand, error) {
	c := publicCommand{}
	if len(args) == 0 {
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
	switch c.name {
	case "key", "mouse", "media", "multi", "rgb":
	default:
		return c, fmt.Errorf("unknown command %q; use key, mouse, media, multi or rgb", c.name)
	}
	var positional []string
	options := map[string]int{}
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
			var err error
			args, err = parseActionOption(c.name, arg, args, options)
			if err != nil {
				return c, err
			}
		default:
			positional = append(positional, arg)
		}
	}
	return c.parsePositionals(positional, options, on, seenOn)
}

func (c publicCommand) parsePositionals(positional []string, options map[string]int, on string, seenOn bool) (publicCommand, error) {
	if len(positional) == 0 {
		if seenOn {
			return c, fmt.Errorf("--on requires a key combination")
		}
		if len(options) > 0 {
			return c, fmt.Errorf("%s options require an action", c.name)
		}
		if c.help {
			return c, nil
		}
		switch c.name {
		case "key":
			return c, fmt.Errorf("key requires a key combination, for example \"wonkey key f13\"; run \"wonkey key --help\" for supported keys")
		case "mouse":
			return c, fmt.Errorf("mouse requires buttons or wheel direction, for example \"wonkey mouse left\"; run \"wonkey mouse --help\" for options")
		case "media":
			return c, fmt.Errorf("media requires a media action, for example \"wonkey media volumeup\"; run \"wonkey media --help\" for actions")
		case "multi":
			return c, fmt.Errorf("multi requires a comma-separated key list, for example \"wonkey multi ctrl,c\"; run \"wonkey multi --help\" for options")
		default:
			return c, fmt.Errorf("rgb requires a lighting mode, for example \"wonkey rgb static 0000ff\" or \"wonkey rgb off\"; run \"wonkey rgb --help\" for modes")
		}
	}
	if c.name == "mouse" || c.name == "media" || c.name == "multi" {
		return c.parseAction(positional, options)
	}
	return c.parseValues(positional, on)
}

func parseActionOption(command, arg string, args []string, options map[string]int) ([]string, error) {
	option, value, equals := strings.Cut(arg, "=")
	min, max := -127, 127
	switch {
	case command == "mouse" && (option == "--x" || option == "--y" || option == "--wheel"):
	case command == "multi" && option == "--interval":
		min, max = 1, 65535
	case command == "multi" && option == "--repeat":
		min, max = 1, 255
	default:
		return args, fmt.Errorf("unknown option %q", arg)
	}
	if _, seen := options[option]; seen {
		return args, fmt.Errorf("%s is accepted once", option)
	}
	if !equals && len(args) > 0 {
		value, args = args[0], args[1:]
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return args, fmt.Errorf("%s needs an integer from %d to %d", option, min, max)
	}
	options[option] = n
	return args, nil
}

func (c publicCommand) parseAction(positional []string, options map[string]int) (publicCommand, error) {
	if len(positional) != 1 {
		return c, fmt.Errorf("%s accepts one action", c.name)
	}
	value := strings.ToLower(positional[0])
	switch c.name {
	case "mouse":
		a := MouseAction{X: options["--x"], Y: options["--y"], Wheel: options["--wheel"]}
		switch value {
		case "wheelup", "wheeldown":
			if _, present := options["--wheel"]; present {
				return c, fmt.Errorf("%s cannot be combined with --wheel", value)
			}
			a.Wheel = 1
			if value == "wheeldown" {
				a.Wheel = -1
			}
		case "none":
		default:
			for button := range strings.SplitSeq(value, "+") {
				bit := map[string]int{"left": 1, "right": 2, "middle": 4}[button]
				if bit == 0 || a.Buttons&bit != 0 {
					return c, fmt.Errorf("invalid or repeated mouse button %q; use left, right, middle or none", button)
				}
				a.Buttons |= bit
			}
		}
		c.action = a
	case "media":
		usage, ok := mediaValues[value]
		if !ok && len(value) == 6 && strings.HasPrefix(value, "0x") {
			b, err := hex.DecodeString(value[2:])
			if err == nil {
				usage, ok = int(b[0])<<8|int(b[1]), true
			}
		}
		if !ok {
			return c, fmt.Errorf("unsupported media action %q; use wonkey media --help", value)
		}
		c.action = MediaAction{Usage: usage}
	case "multi":
		a := MultiAction{Interval: 50, Repeat: 1}
		if n, present := options["--interval"]; present {
			a.Interval = n
		}
		if n, present := options["--repeat"]; present {
			a.Repeat = n
		}
		keys := strings.Split(value, ",")
		if len(keys) > 115 {
			return c, fmt.Errorf("multi requires 1..115 keys")
		}
		for _, key := range keys {
			code, ok := keyValues[canonicalKeyName(key)]
			if !ok {
				return c, fmt.Errorf("unsupported multi key %q; use wonkey key --help", key)
			}
			a.Keys = append(a.Keys, byte(code&255))
		}
		c.action = a
	}
	_, err := c.action.actionBytes()
	return c, err
}

func (c publicCommand) parseValues(positional []string, on string) (publicCommand, error) {
	var err error
	if c.name == "key" {
		if len(positional) != 1 {
			return c, fmt.Errorf("key accepts one complete combination")
		}
		parts := strings.Split(strings.ToLower(positional[0]), "+")
		parts[len(parts)-1] = canonicalKeyName(parts[len(parts)-1])
		if _, ok := keyValues[parts[len(parts)-1]]; !ok {
			return c, fmt.Errorf("unsupported key %q; use wonkey key --help for supported names", parts[len(parts)-1])
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
		mode := strings.ToLower(positional[0])
		if _, ok := lightingValues[mode]; !ok {
			return c, fmt.Errorf("unknown RGB mode %q", positional[0])
		}
		colour := ""
		if len(positional) == 2 {
			if mode == "cycle-slow" || mode == "cycle-fast" || mode == "off" {
				return c, fmt.Errorf("RGB mode %q does not use a colour; omit RGB", mode)
			}
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
	discoverCH57 func(string) ([]ch57.DiscoveredCandidate, error)
	probeCH57    func(string, string, ch57.DiscoveredCandidate) (ch57.ProbeResult, error)
	discover     func() ([]Candidate, error)
	query        func(Candidate) (CaptureResult, error)
	apply        func(Candidate, ApplyTarget, configurationChanges, func(configuration, configuration) (bool, error)) (ApplyResult, error)
	interactive  func(io.Reader) bool
}

func publicDeviceAccess() publicAccess {
	return publicAccess{
		discoverCH57: func(sysRoot string) ([]ch57.DiscoveredCandidate, error) {
			if reflect.ValueOf(liveDiscover).Pointer() != reflect.ValueOf(discover).Pointer() {
				return nil, nil
			}
			return ch57.DiscoverCandidates(sysRoot)
		},
		probeCH57: ch57.ProbeCandidate,
		discover:  func() ([]Candidate, error) { return liveDiscover("/sys/bus/usb/devices", "/dev") },
		query:     func(selected Candidate) (CaptureResult, error) { return querySelected(selected, true) },
		apply: func(selected Candidate, target ApplyTarget, changes configurationChanges, guard func(configuration, configuration) (bool, error)) (ApplyResult, error) {
			return liveApplyBound(selected.PhysicalPath, target, changes, true, guard, &selected, true)
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
	out := rt.errOut
	if err == nil {
		code = 1
		out = rt.out
	}
	h := newHuman(out)
	h.header()
	if h.out.err != nil {
		return &CLIError{Err: h.out.err, Code: code}
	}
	if err == nil {
		if c.help {
			text := publicHelp
			if c.name == "key" {
				text = keyHelp
			}
			if c.name == "rgb" {
				text = rgbHelp
			}
			switch c.name {
			case "mouse":
				text = mouseHelp
			case "media":
				text = mediaHelp
			case "multi":
				text = multiHelp
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
		product := c.Product
		if product == "" {
			product = "XFKEY One Key Max (0112)"
		}
		h.field(strconv.Itoa(i+1), product+" "+c.PhysicalPath+" ("+c.VendorNode.Path+")")
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

func (c publicCommand) isReadOnly() bool {
	return len(c.changes) == 0 && c.action == nil
}

func (c publicCommand) run(rt *cliRuntime, access publicAccess) error {
	interactive := access.interactive(rt.in)
	if !c.isReadOnly() && !interactive {
		return fmt.Errorf("changes require an interactive terminal; no device access")
	}
	input := bufio.NewReader(rt.in)
	h := newHuman(rt.out)
	const sysRoot, devRoot = "/sys/bus/usb/devices", "/dev/bus/usb"
	var candidates []Candidate
	if access.discover != nil {
		var err error
		candidates, err = access.discover()
		if err != nil {
			return err
		}
	}
	var chCandidates []ch57.DiscoveredCandidate
	if access.discoverCH57 != nil {
		var err error
		chCandidates, err = access.discoverCH57(sysRoot)
		if err != nil {
			return err
		}
	}
	byCH57Path := make(map[string]ch57.DiscoveredCandidate, len(chCandidates))
	for _, candidate := range chCandidates {
		candidates = append(candidates, Candidate{
			PhysicalPath: candidate.PhysicalPath, Product: "CH57x Macropad (1189:8890)", Compatible: true,
			VendorNode: NodeMetadata{Path: fmt.Sprintf("%s/%03d/%03d", devRoot, candidate.BusNumber, candidate.DeviceNumber)},
		})
		byCH57Path[candidate.PhysicalPath] = candidate
	}
	if c.isReadOnly() {
		return c.runReadAll(candidates, byCH57Path, h, access)
	}
	selected, err := choosePublic(candidates, input, h, interactive)
	if err != nil {
		return err
	}
	if selected == nil {
		if len(chCandidates) > 0 && len(candidates) == len(chCandidates) {
			h.line("", "Cancelled. No USB device opened.")
		} else {
			h.line("", "Cancelled. No HID device opened.")
		}
		return h.out.err
	}
	if _, ok := byCH57Path[selected.PhysicalPath]; ok {
		return fmt.Errorf("live writes are not supported for 1189:8890; no settings write sent")
	}
	return c.runXFKEYSelected(*selected, input, h, access)
}

func (c publicCommand) runReadAll(candidates []Candidate, byCH57Path map[string]ch57.DiscoveredCandidate, h human, access publicAccess) error {
	var matches []Candidate
	for _, candidate := range candidates {
		if candidate.Compatible {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no compatible device found")
	}
	for i, selected := range matches {
		if i > 0 {
			fmt.Fprintln(h.out)
		}
		if chCandidate, ok := byCH57Path[selected.PhysicalPath]; ok {
			if err := c.runCH57Selected(chCandidate, h, access); err != nil {
				return err
			}
			continue
		}
		if err := c.runXFKEYSelected(selected, nil, h, access); err != nil {
			return err
		}
	}
	return h.out.err
}

func (c publicCommand) runXFKEYSelected(selected Candidate, input *bufio.Reader, h human, access publicAccess) error {
	observed, err := access.query(selected)
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
	return c.runObserved(observed, selected, target, input, h, access)
}

func (c publicCommand) runCH57Selected(selected ch57.DiscoveredCandidate, h human, access publicAccess) error {
	const sysRoot, devRoot = "/sys/bus/usb/devices", "/dev/bus/usb"
	h.heading("CH57x Macropad (1189:8890)")
	h.field("USB path", selected.PhysicalPath)
	h.field("Layout", "3-key / 1-knob up to 12-key / 3-knob")
	if h.out.err != nil {
		return h.out.err
	}
	result, err := access.probeCH57(sysRoot, devRoot, selected)
	shortAA := err == nil && result.Count == 1 && len(result.Raw) == 1 && result.Raw[0] == 0xaa
	if err == nil {
		if shortAA {
			h.field("Status", "Connected (write-only firmware)")
			h.field("Settings", "Not readable over USB")
		} else if result.Status == ch57.ProbeRawReport {
			h.field("Status", "Connected")
			h.field("Settings", "Not readable over USB")
			h.field("Input hex", hex.EncodeToString(result.Raw))
		}
		return h.out.err
	}
	h.field("Status", string(result.Status))
	if result.Raw != nil || result.Count != 0 || result.Status == ch57.ProbeTransferFailed {
		h.field("Input bytes", strconv.Itoa(result.Count))
		h.field("Input hex", hex.EncodeToString(result.Raw))
	}
	return errors.Join(err, h.out.err)
}

func (c publicCommand) showCurrentSettings(current configuration, selected Candidate, target ApplyTarget, h human) {
	h.heading("XFKEY One Key Max (0112)")
	h.field("USB path", selected.PhysicalPath)
	h.field("Identifier", target.Identifier)
	h.field("Version", target.Version)
	if c.supportedConfiguration(current) != nil {
		h.field("Layout", "Unknown (unsupported configuration)")
		h.field("Raw type", fmt.Sprintf("0x%02x (byte 0)", current[0]))
		h.field("Raw 0-15", hex.EncodeToString(current[:16]))
		h.line("33", "Fields are not decoded. Changes are blocked.")
		return
	}
	if c.name != "rgb" {
		action, _ := decodeAction(current)
		if keyboard, ok := action.(KeyboardAction); ok {
			key := settingName(keyValues, keyboard.Key)
			if keyboard.Modifiers != 0 {
				key = strings.ReplaceAll(modifierName(current[2]), ",", "+") + "+" + key
			}
			h.field("Key", key)
			h.field("On", settingName(triggerValues, keyboard.Trigger))
		} else {
			h.field("Action", actionDescription(action))
		}
	}
	if c.name == "rgb" || c.name == "" {
		h.field("Mode", settingName(lightingValues, int(current[124])-1))
		fmt.Fprintf(h.out, "  %-10s %s\n", "Colour", h.rgb(fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127])))
	}
}

func (c publicCommand) supportedConfiguration(current configuration) error {
	if _, err := decodeAction(current); err != nil {
		return fmt.Errorf("unsupported current action layout: %w", err)
	}
	if current[124] < 1 || current[124] > 8 {
		return fmt.Errorf("unsupported current RGB mode %02x", current[124])
	}
	return nil
}

func (c publicCommand) configurationChanges(current configuration) configurationChanges {
	if c.action != nil {
		return ActionChanges{Action: c.action}
	}
	if current[0] == 0 {
		return c.changes
	}
	if c.name == "key" {
		trigger, present := c.changes["trigger"]
		if !present {
			trigger = triggerValues["press"]
		}
		return ActionChanges{Action: KeyboardAction{Trigger: trigger, Modifiers: c.changes["modifiers"], Key: c.changes["key"]}}
	}
	return ActionChanges{RGB: c.changes}
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
	if len(c.changes) == 0 && c.action == nil {
		c.showCurrentSettings(current, selected, target, h)
		return h.out.err
	}
	if err := c.supportedConfiguration(current); err != nil {
		return err
	}
	c.showCurrentSettings(current, selected, target, h)
	if h.out.err != nil {
		return h.out.err
	}
	changes := c.configurationChanges(current)
	intended, err := changes.configuration(current)
	if err != nil {
		return err
	}
	views := publicActionViews(current, intended)
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
	guard := func(fresh, proposed configuration) (bool, error) {
		if fresh != current || proposed != intended {
			return false, fmt.Errorf("settings changed after preview; no settings write sent")
		}
		return true, nil
	}
	result, err := access.apply(selected, target, changes, guard)
	if err != nil {
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
	return h.out.err
}
