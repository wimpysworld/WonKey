package xfkey

import (
	"bufio"
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"wonkey/internal/ch57"
)

const publicHelp = `Inspect and configure supported macropads.

Usage: wonkey [command] [options]

Without a command, inspect all connected supported devices.

Commands:
  key [CONTROL] <COMBINATION>   Change the key.
  trigger <WHEN>                Change when the key fires.
  macro [CONTROL] <STROKE,...>  Change the macro key sequence.
  mouse [CONTROL] <BUTTONS>     Change the mouse action.
  media [CONTROL] <ACTION>      Change the media action.
  rgb <MODE> [RGB]              Change the RGB lighting.
  led <MODE>                    Change the LED backlight mode.

Options:
  --layer N                     Select layer 1..15 on multi-layer devices (default: 1).
  --device N                    Select device N when multiple devices are connected.
  -h, --help                    Show help without device access.

Examples:
  wonkey
` + `  wonkey key f13
  wonkey key key2 ctrl+shift+f13
  wonkey trigger release
  wonkey macro ctrl+c,ctrl+v
  wonkey media knob-right volumeup
  wonkey rgb static 0000ff
  wonkey led on

Run "wonkey <command> --help" for values and options.
`

const keyHelp = `Usage: wonkey key [CONTROL] <COMBINATION> [--layer N] [--device N]

Run "wonkey" without arguments to inspect connected devices.
Controls (multi-control devices, default: key1): key1-key12, knob-left, knob-press, knob-right, knob1-left..knob3-right
A combination contains exactly one base key from the names below.
Optional modifiers precede the key: ctrl+, shift+, alt+ or super+, each at most once.
Key names ignore case. Uppercase letters do not add Shift.
Unspecified modifiers are cleared. An existing keyboard trigger is preserved.
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
  --layer N     Layer 1..15 on multi-layer devices (default: 1).
  --device N    Select device N when multiple devices are connected.
  -h, --help    Show help without device access.

Examples:
  wonkey key f13
  wonkey key key2 ctrl+shift+f13

At [Y/n], Enter accepts. No or EOF cancels.
`

const triggerHelp = `Usage: wonkey trigger <press|release|both> [--device N]

Run "wonkey" without arguments to inspect connected devices.
Set when the configured key fires on devices that support key triggers.
Requires the device to have a keyboard key configured.

Values:
  press      Fire when the key is pressed
  release    Fire when the key is released
  both       Fire on both press and release

Options:
  --device N   Select device N when multiple devices are connected.
  -h, --help   Show help without device access.

Examples:
  wonkey trigger press
  wonkey trigger release

At [Y/n], Enter accepts. No or EOF cancels.
`

const rgbHelp = `Usage: wonkey rgb <MODE> [RGB] [--device N]

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
  --device N    Select device N when multiple devices are connected.
  -h, --help    Show help without device access.

Examples:
  wonkey rgb static 0000ff
  wonkey rgb static
  wonkey rgb off

At [Y/n], Enter accepts. No or EOF cancels.
`

const mouseHelp = `Usage: wonkey mouse [CONTROL] <BUTTONS> [--x N] [--y N] [--wheel N] [--layer N] [--device N]

Run "wonkey" without arguments to inspect connected devices.
Controls (multi-control devices, default: key1): key1-key12, knob-left, knob-press, knob-right, knob1-left..knob3-right
Buttons: left, right, middle, none. Join buttons with +, for example left+right.
Use none alone for movement without a button, or a zero action.
Use wheelup or wheeldown alone for one wheel tick. Do not combine with --wheel.
X, Y and wheel accept -127..127. Omitted values are zero.

Options:
  --layer N     Layer 1..15 on multi-layer devices (default: 1).
  --device N    Select device N when multiple devices are connected.
  -h, --help    Show help without device access.

Examples:
  wonkey mouse left
  wonkey mouse knob1-press middle

At [Y/n], Enter accepts. No or EOF cancels.
`

const mediaHelp = `Usage: wonkey media [CONTROL] <ACTION|0xHHHH> [--layer N] [--device N]

Run "wonkey" without arguments to inspect connected devices.
Controls (multi-control devices, default: key1): key1-key12, knob-left, knob-press, knob-right, knob1-left..knob3-right
Names ignore case:
  play, pause, record, fastforward, rewind, next, prev, previous, stop, eject
  playpause, mute, volumeup, volumedown, calculator, mycomputer, browser, email
  search, home, back, forward, refresh, bookmarks
Raw usages need exactly four hexadecimal digits and must be 0x0001..0x023c.

Options:
  --layer N     Layer 1..15 on multi-layer devices (default: 1).
  --device N    Select device N when multiple devices are connected.
  -h, --help    Show help without device access.

Examples:
  wonkey media volumeup
  wonkey media knob-right volumeup

At [Y/n], Enter accepts. No or EOF cancels.
`

const macroHelp = `Usage: wonkey macro [CONTROL] <STROKE[,STROKE...]> [--interval MS] [--repeat COUNT] [--layer N] [--device N]

Run "wonkey" without arguments to inspect connected devices.
Controls (multi-control devices, default: key1): key1-key12, knob-left, knob-press, knob-right, knob1-left..knob3-right
Use 1..115 comma-separated key strokes using base key names from "wonkey key --help".
On devices that support per-stroke modifiers, prefix strokes with ctrl+, shift+, alt+ or super+.
--interval accepts 1..65535 milliseconds (default: 50).
--repeat accepts 1..255 repetitions (default: 1).

Options:
  --layer N     Layer 1..15 on multi-layer devices (default: 1).
  --device N    Select device N when multiple devices are connected.
  -h, --help    Show help without device access.

Examples:
  wonkey macro a,b,c
  wonkey macro key3 ctrl+c,ctrl+v

At [Y/n], Enter accepts. No or EOF cancels.
`

const ledHelp = `Usage: wonkey led <MODE> [--device N]

Run "wonkey" without arguments to inspect connected devices.
Configure the LED backlight mode on devices that support LED modes.

Modes:
  off            Backlight off (mode 0)
  on, static     Steady backlight (mode 1)
  0..255         Raw LED mode number

Options:
  --device N   Select device N when multiple devices are connected.
  -h, --help   Show help without device access.

Examples:
  wonkey led on
  wonkey led off
  wonkey led 2

At [Y/n], Enter accepts. No or EOF cancels.
`

type publicCommand struct {
	name                       string
	control                    ch57.Control
	seenControl                bool
	layer                      int
	seenLayer                  bool
	changes                    Changes
	action                     Action
	device                     int
	ledMode                    int
	hasLED                     bool
	hasMultiOptions            bool
	hasMacroModifiers          bool
	hasUnsupportedCH57MacroKey bool
	macroCount                 int
	help                       bool
}

func parseControlName(raw string) (ch57.Control, bool) {
	c := ch57.Control(strings.ToLower(raw))
	switch c {
	case ch57.Key1, ch57.Key2, ch57.Key3, ch57.Key4, ch57.Key5, ch57.Key6,
		ch57.Key7, ch57.Key8, ch57.Key9, ch57.Key10, ch57.Key11, ch57.Key12,
		ch57.KnobLeft, ch57.KnobPress, ch57.KnobRight,
		ch57.Knob1Left, ch57.Knob1Press, ch57.Knob1Right,
		ch57.Knob2Left, ch57.Knob2Press, ch57.Knob2Right,
		ch57.Knob3Left, ch57.Knob3Press, ch57.Knob3Right:
		return c, true
	default:
		return "", false
	}
}

func parsePublic(args []string) (publicCommand, error) {
	c := publicCommand{control: ch57.Key1, layer: 1}
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
	if args[0] == "--layer" || strings.HasPrefix(args[0], "--layer=") {
		return c, fmt.Errorf("--layer is supported on key, macro, mouse and media only")
	}
	c.name = args[0]
	switch c.name {
	case "key", "trigger", "macro", "mouse", "media", "rgb", "led":
	default:
		return c, fmt.Errorf("unknown command %q; use key, trigger, macro, mouse, media, rgb or led", c.name)
	}
	var positional []string
	options := map[string]int{}
	args = args[1:]
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		switch {
		case arg == "--help" || arg == "-h":
			c.help = true
		case arg == "--layer" || strings.HasPrefix(arg, "--layer="):
			if c.name != "key" && c.name != "macro" && c.name != "mouse" && c.name != "media" {
				return c, fmt.Errorf("--layer is supported on key, macro, mouse and media only")
			}
			if c.seenLayer {
				return c, fmt.Errorf("--layer is accepted once")
			}
			var value string
			if arg == "--layer" {
				if len(args) == 0 {
					return c, fmt.Errorf("--layer needs 1..15")
				}
				value, args = args[0], args[1:]
			} else {
				value = strings.TrimPrefix(arg, "--layer=")
			}
			layer, ok := parseBoundedInt(value, 1, 15)
			if !ok {
				return c, fmt.Errorf("--layer needs 1..15")
			}
			c.layer = layer
			c.seenLayer = true
		case arg == "--device" || strings.HasPrefix(arg, "--device="):
			if c.device != 0 {
				return c, fmt.Errorf("--device is accepted once")
			}
			var value string
			if arg == "--device" {
				if len(args) == 0 {
					return c, fmt.Errorf("--device needs a positive device number")
				}
				value, args = args[0], args[1:]
			} else {
				value = strings.TrimPrefix(arg, "--device=")
			}
			device, err := parseDeviceNumber(value)
			if err != nil {
				return c, err
			}
			c.device = device
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
	return c.parsePositionals(positional, options)
}

func parseDeviceNumber(value string) (int, error) {
	if len(value) == 0 || value[0] < '1' || value[0] > '9' {
		return 0, fmt.Errorf("--device needs a positive device number")
	}
	for i := 1; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, fmt.Errorf("--device needs a positive device number")
		}
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("--device needs a positive device number")
	}
	return n, nil
}

func parseBoundedInt(value string, min, max int) (int, bool) {
	if len(value) == 0 || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return 0, false
	}
	return n, true
}

func missingCommandValueError(name string) error {
	switch name {
	case "key":
		return fmt.Errorf("key requires a key combination, for example \"wonkey key f13\"; run \"wonkey key --help\" for supported keys")
	case "trigger":
		return fmt.Errorf("trigger requires press, release or both, for example \"wonkey trigger release\"; run \"wonkey trigger --help\" for values")
	case "macro":
		return fmt.Errorf("macro requires a comma-separated key list, for example \"wonkey macro ctrl+c,ctrl+v\" or \"wonkey macro a,b,c\"; run \"wonkey macro --help\" for options")
	case "mouse":
		return fmt.Errorf("mouse requires buttons or wheel direction, for example \"wonkey mouse left\"; run \"wonkey mouse --help\" for options")
	case "media":
		return fmt.Errorf("media requires a media action, for example \"wonkey media volumeup\"; run \"wonkey media --help\" for actions")
	case "led":
		return fmt.Errorf("led requires a mode, for example \"wonkey led on\" or \"wonkey led off\"; run \"wonkey led --help\" for modes")
	default:
		return fmt.Errorf("rgb requires a lighting mode, for example \"wonkey rgb static 0000ff\" or \"wonkey rgb off\"; run \"wonkey rgb --help\" for modes")
	}
}

func (c publicCommand) parsePositionals(positional []string, options map[string]int) (publicCommand, error) {
	if (c.name == "key" || c.name == "macro" || c.name == "mouse" || c.name == "media") && len(positional) > 0 {
		if control, ok := parseControlName(positional[0]); ok {
			c.control = control
			c.seenControl = true
			positional = positional[1:]
			if len(positional) == 0 {
				return c, missingCommandValueError(c.name)
			}
		}
	}
	if len(positional) == 0 {
		if c.device > 0 {
			return c, fmt.Errorf("--device requires a command value")
		}
		if c.seenLayer {
			return c, fmt.Errorf("--layer requires a command value")
		}
		if len(options) > 0 {
			return c, fmt.Errorf("%s options require an action", c.name)
		}
		if c.help {
			return c, nil
		}
		return c, missingCommandValueError(c.name)
	}
	if c.name == "mouse" || c.name == "media" || c.name == "macro" {
		return c.parseAction(positional, options)
	}
	return c.parseValues(positional, options)
}

func parseActionOption(command, arg string, args []string, options map[string]int) ([]string, error) {
	option, value, equals := strings.Cut(arg, "=")
	min, max := -127, 127
	switch {
	case command == "mouse" && (option == "--x" || option == "--y" || option == "--wheel"):
	case command == "macro" && option == "--interval":
		min, max = 1, 65535
	case command == "macro" && option == "--repeat":
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

func parseMacroStroke(stroke string) (ch57.Stroke, error) {
	if stroke == "" {
		return ch57.Stroke{}, fmt.Errorf("unsupported macro key %q; use wonkey key --help", stroke)
	}
	parts := strings.Split(stroke, "+")
	seen := map[string]bool{}
	modifiers := 0
	for i, part := range parts {
		bit, isMod := map[string]int{"ctrl": 1, "shift": 2, "alt": 4, "super": 8}[part]
		if isMod {
			if seen[part] {
				return ch57.Stroke{}, fmt.Errorf("invalid modifier %q", part)
			}
			seen[part] = true
			modifiers |= bit
			continue
		}
		if i != len(parts)-1 {
			return ch57.Stroke{}, fmt.Errorf("invalid modifier %q", part)
		}
		code, ok := keyValues[canonicalKeyName(part)]
		if !ok {
			return ch57.Stroke{}, fmt.Errorf("unsupported macro key %q; use wonkey key --help", stroke)
		}
		return ch57.Stroke{Modifiers: modifiers, Usage: code}, nil
	}
	return ch57.Stroke{Modifiers: modifiers, Usage: 0}, nil
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
	case "macro":
		c.hasMultiOptions = len(options) > 0
		a := MultiAction{Interval: 50, Repeat: 1}
		if n, present := options["--interval"]; present {
			a.Interval = n
		}
		if n, present := options["--repeat"]; present {
			a.Repeat = n
		}
		strokes := strings.Split(value, ",")
		if len(strokes) > 115 {
			return c, fmt.Errorf("macro accepts 1..115 comma-separated keys")
		}
		c.macroCount = len(strokes)
		for _, raw := range strokes {
			stroke, err := parseMacroStroke(raw)
			if err != nil {
				return c, err
			}
			if stroke.Modifiers != 0 || stroke.Usage == 0 {
				c.hasMacroModifiers = true
			}
			if stroke.Usage != 0 && (stroke.Usage < 0x04 || stroke.Usage > 0x73) {
				c.hasUnsupportedCH57MacroKey = true
			}
			if !c.hasMacroModifiers {
				a.Keys = append(a.Keys, byte(stroke.Usage&255))
			}
		}
		if c.hasMacroModifiers {
			return c, nil
		}
		c.action = a
	}
	_, err := c.action.actionBytes()
	return c, err
}

func (c publicCommand) parseValues(positional []string, options map[string]int) (publicCommand, error) {
	var err error
	switch c.name {
	case "key":
		if len(options) > 0 {
			return c, fmt.Errorf("key accepts no action options")
		}
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
		c.changes, err = parseSettings(parts[len(parts)-1], "", modifiers, "", "")
	case "trigger":
		if len(options) > 0 {
			return c, fmt.Errorf("trigger accepts no action options")
		}
		if len(positional) != 1 {
			return c, fmt.Errorf("trigger requires exactly one value: press, release or both")
		}
		value, ok := triggerValues[strings.ToLower(positional[0])]
		if !ok {
			return c, fmt.Errorf("trigger needs press, release or both")
		}
		c.changes = Changes{"trigger": value}
		return c, nil
	case "led":
		if len(options) > 0 {
			return c, fmt.Errorf("led accepts no action options")
		}
		if len(positional) != 1 {
			return c, fmt.Errorf("led accepts one mode argument, not RGB colours; use \"wonkey rgb\" on RGB devices")
		}
		mode := strings.ToLower(positional[0])
		switch mode {
		case "off":
			c.ledMode = 0
		case "on", "static":
			c.ledMode = 1
		default:
			n, ok := parseBoundedInt(mode, 0, 255)
			if !ok {
				return c, fmt.Errorf("invalid led mode %q; use off, on, static or 0..255", positional[0])
			}
			c.ledMode = n
		}
		c.hasLED = true
		return c, nil
	default:
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
			switch c.name {
			case "key":
				text = keyHelp
			case "trigger":
				text = triggerHelp
			case "macro":
				text = macroHelp
			case "mouse":
				text = mouseHelp
			case "media":
				text = mediaHelp
			case "rgb":
				text = rgbHelp
			case "led":
				text = ledHelp
			}
			return printHumanHelp(rt.out, text)
		}
		err = c.run(rt, access)
	}
	if err != nil {
		errH := newHuman(rt.errOut)
		lines := strings.Split(err.Error(), "\n")
		if len(lines) == 1 {
			errH.line("31", "Error: "+lines[0]+".")
		} else {
			errH.line("31", "Error: "+lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintln(errH.out, escapeHuman(line))
			}
		}
		fmt.Fprintln(rt.errOut, `Run "wonkey --help" for commands.`)
		return &CLIError{Err: err, Code: code}
	}
	return nil
}

func comparePhysicalPath(a, b string) int {
	split := func(r rune) bool { return r == '-' || r == '.' }
	sa, sb := strings.FieldsFunc(a, split), strings.FieldsFunc(b, split)
	for i := range min(len(sa), len(sb)) {
		na, errA := strconv.Atoi(sa[i])
		nb, errB := strconv.Atoi(sb[i])
		switch {
		case errA == nil && errB == nil:
			if c := cmp.Compare(na, nb); c != 0 {
				return c
			}
			if c := strings.Compare(sa[i], sb[i]); c != 0 {
				return c
			}
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		default:
			if c := strings.Compare(sa[i], sb[i]); c != 0 {
				return c
			}
		}
	}
	if c := cmp.Compare(len(sa), len(sb)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func compatibleCandidates(candidates []Candidate) []Candidate {
	var matches []Candidate
	for _, candidate := range candidates {
		if candidate.Compatible {
			matches = append(matches, candidate)
		}
	}
	slices.SortFunc(matches, func(a, b Candidate) int {
		return comparePhysicalPath(a.PhysicalPath, b.PhysicalPath)
	})
	return matches
}

func (c publicCommand) selectWriteDevice(matches []Candidate) (Candidate, int, error) {
	if c.device == 0 {
		if len(matches) == 1 {
			return matches[0], 1, nil
		}
		var b strings.Builder
		b.WriteString("multiple compatible devices connected; specify --device <N>:")
		for i, m := range matches {
			product := m.Product
			if product == "" {
				product = "XFKEY One Key Max (0112)"
			}
			fmt.Fprintf(&b, "\n  --device %d  %s (USB path %s)", i+1, product, m.PhysicalPath)
		}
		return Candidate{}, 0, errors.New(b.String())
	}
	if c.device > len(matches) {
		return Candidate{}, 0, fmt.Errorf("device %d not found (%d compatible device(s) connected)", c.device, len(matches))
	}
	return matches[c.device-1], c.device, nil
}

func (c publicCommand) isReadOnly() bool {
	return c.name == "" && len(c.changes) == 0 && c.action == nil && !c.hasLED && !c.hasMacroModifiers
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
	matches := compatibleCandidates(candidates)
	if len(matches) == 0 {
		return fmt.Errorf("no compatible device found")
	}
	if c.isReadOnly() {
		return c.runReadAll(matches, byCH57Path, h, access)
	}
	selected, deviceID, err := c.selectWriteDevice(matches)
	if err != nil {
		return err
	}
	if _, ok := byCH57Path[selected.PhysicalPath]; ok {
		if err := c.checkCH57Capabilities(); err != nil {
			return err
		}
		return fmt.Errorf("live writes are not supported for 1189:8890; no settings write sent")
	}
	if err := c.checkXFKEYCapabilities(); err != nil {
		return err
	}
	return c.runXFKEYSelected(selected, deviceID, input, h, access)
}

func (c publicCommand) checkXFKEYCapabilities() error {
	if c.seenLayer {
		return fmt.Errorf("XFKEY One Key Max (0112) does not support --layer")
	}
	if c.seenControl {
		return fmt.Errorf("XFKEY One Key Max (0112) has a single key and does not support %s", c.control)
	}
	switch c.name {
	case "led":
		return fmt.Errorf("XFKEY One Key Max (0112) does not support led; use \"wonkey rgb\" instead")
	case "macro":
		if c.hasMacroModifiers {
			return fmt.Errorf("XFKEY One Key Max (0112) does not support modifiers in macro sequences; use base key names only")
		}
	}
	return nil
}

func (c publicCommand) checkCH57Capabilities() error {
	switch c.name {
	case "key", "macro", "mouse", "media":
		if _, ok := parseControlName(string(c.control)); !ok {
			return fmt.Errorf("CH57x Macropad (1189:8890) does not support control %q", c.control)
		}
		if c.layer < 1 || c.layer > 15 {
			return fmt.Errorf("CH57x Macropad (1189:8890) supports layers 1..15 only")
		}
	}
	switch c.name {
	case "trigger":
		return fmt.Errorf("CH57x Macropad (1189:8890) does not support trigger (key release triggers)")
	case "rgb":
		return fmt.Errorf("CH57x Macropad (1189:8890) does not support rgb; use \"wonkey led\" instead")
	case "key":
		if c.changes["key"] < 0x04 || c.changes["key"] > 0x73 {
			return fmt.Errorf("CH57x Macropad (1189:8890) supports keys a-z, 0-9, and f1-f24 (usages 0x04..0x73) only")
		}
	case "macro":
		if c.hasMultiOptions {
			return fmt.Errorf("CH57x Macropad (1189:8890) does not support --interval or --repeat")
		}
		if c.macroCount > 5 {
			return fmt.Errorf("CH57x Macropad (1189:8890) supports up to 5 strokes per macro")
		}
		if c.hasUnsupportedCH57MacroKey {
			return fmt.Errorf("CH57x Macropad (1189:8890) supports keys a-z, 0-9, and f1-f24 (usages 0x04..0x73) only")
		}
	case "media":
		if media, ok := c.action.(MediaAction); ok {
			switch ch57.MediaUsage(media.Usage) {
			case ch57.MediaScreenBrightnessUp, ch57.MediaScreenBrightnessDown,
				ch57.MediaNext, ch57.MediaPrevious, ch57.MediaStop, ch57.MediaPlay, ch57.MediaMute,
				ch57.MediaVolumeUp, ch57.MediaVolumeDown, ch57.MediaFavorites, ch57.MediaCalculator, ch57.MediaScreenLock,
				ch57.MediaWebPageHome, ch57.MediaWebPageBack, ch57.MediaWebPageForward:
			default:
				return fmt.Errorf("CH57x Macropad (1189:8890) does not support that media action")
			}
		}
	}
	return nil
}

func (c publicCommand) runReadAll(matches []Candidate, byCH57Path map[string]ch57.DiscoveredCandidate, h human, access publicAccess) error {
	for i, selected := range matches {
		deviceID := i + 1
		if i > 0 {
			fmt.Fprintln(h.out)
		}
		if chCandidate, ok := byCH57Path[selected.PhysicalPath]; ok {
			if err := c.runCH57Selected(chCandidate, deviceID, h, access); err != nil {
				return err
			}
			continue
		}
		if err := c.runXFKEYSelected(selected, deviceID, nil, h, access); err != nil {
			return err
		}
	}
	return h.out.err
}

func (c publicCommand) runXFKEYSelected(selected Candidate, deviceID int, input *bufio.Reader, h human, access publicAccess) error {
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
	return c.runObserved(observed, selected, deviceID, target, input, h, access)
}

func (c publicCommand) runCH57Selected(selected ch57.DiscoveredCandidate, deviceID int, h human, access publicAccess) error {
	const sysRoot, devRoot = "/sys/bus/usb/devices", "/dev/bus/usb"
	h.heading(fmt.Sprintf("[%d] CH57x Macropad (1189:8890)", deviceID))
	h.field("USB path", selected.PhysicalPath)
	h.field("Supports", "key, mouse, media, macro, led")
	if h.out.err != nil {
		return h.out.err
	}
	result, err := access.probeCH57(sysRoot, devRoot, selected)
	if err == nil {
		if result.Count == 1 && len(result.Raw) == 1 && result.Raw[0] == 0xaa {
			h.field("Settings", "Write-only (not readable over USB)")
			return h.out.err
		}
		if result.Status == ch57.ProbeRawReport {
			h.field("Settings", "Write-only (not readable over USB)")
			h.field("Input hex", hex.EncodeToString(result.Raw))
			return h.out.err
		}
	}
	h.field("Status", string(result.Status))
	if result.ReportDescriptor != nil || result.ReportDescriptorCount != 0 {
		h.field("Descriptor bytes", strconv.Itoa(result.ReportDescriptorCount))
		h.field("Descriptor hex", hex.EncodeToString(result.ReportDescriptor))
	}
	if result.Raw != nil || result.Count != 0 || result.Status == ch57.ProbeTransferFailed {
		h.field("Input bytes", strconv.Itoa(result.Count))
		h.field("Input hex", hex.EncodeToString(result.Raw))
	}
	return errors.Join(err, h.out.err)
}

func (c publicCommand) showCurrentSettings(current configuration, selected Candidate, deviceID int, h human) {
	h.heading(fmt.Sprintf("[%d] XFKEY One Key Max (0112)", deviceID))
	h.field("USB path", selected.PhysicalPath)
	h.field("Supports", "key, trigger, mouse, media, macro, rgb")
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

func (c publicCommand) runObserved(observed CaptureResult, selected Candidate, deviceID int, target ApplyTarget, input *bufio.Reader, h human, access publicAccess) error {
	raw, err := hex.DecodeString(observed.Readback.Configuration)
	if err != nil {
		return err
	}
	var current configuration
	if len(raw) != len(current) {
		return fmt.Errorf("device settings need exactly 128 bytes")
	}
	copy(current[:], raw)
	if c.isReadOnly() {
		c.showCurrentSettings(current, selected, deviceID, h)
		return h.out.err
	}
	if err := c.supportedConfiguration(current); err != nil {
		return err
	}
	if c.name == "trigger" && current[0] != 0 {
		return fmt.Errorf("trigger requires a configured keyboard key; set one with \"wonkey key\" first")
	}
	c.showCurrentSettings(current, selected, deviceID, h)
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
