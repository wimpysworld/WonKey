package trisat

import "fmt"

// MediaUsage lists the consumer usages named in the pinned source.
// Source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/mod.rs
type MediaUsage int

const (
	MediaNext       MediaUsage = 0xb5
	MediaPrevious   MediaUsage = 0xb6
	MediaStop       MediaUsage = 0xb7
	MediaPlay       MediaUsage = 0xcd
	MediaMute       MediaUsage = 0xe2
	MediaVolumeUp   MediaUsage = 0xe9
	MediaVolumeDown MediaUsage = 0xea
	MediaFavorites  MediaUsage = 0x182
	MediaCalculator MediaUsage = 0x192
	MediaScreenLock MediaUsage = 0x19e
)

// EncodeMedia returns a first-layer binding with a 16-bit little-endian usage.
// Encoding source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/k8890.rs
func EncodeMedia(profile Profile, control Control, usage MediaUsage) ([]Report, error) {
	slot, err := profile.slot(control)
	if err != nil {
		return nil, err
	}
	switch usage {
	case MediaNext, MediaPrevious, MediaStop, MediaPlay, MediaMute,
		MediaVolumeUp, MediaVolumeDown, MediaFavorites, MediaCalculator, MediaScreenLock:
	default:
		return nil, fmt.Errorf("unsupported TriSat media usage %#x", usage)
	}
	return bindingReports(Report{0x03, slot, 0x12, byte(usage & 255), byte((usage >> 8) & 255)}), nil
}

type MouseAction string

const (
	MouseClick     MouseAction = "click"
	MouseWheelUp   MouseAction = "wheel-up"
	MouseWheelDown MouseAction = "wheel-down"
)

type MouseButtons int

const (
	MouseLeft   MouseButtons = 0x01
	MouseRight  MouseButtons = 0x02
	MouseMiddle MouseButtons = 0x04
)

// MouseModifier is a single modifier, not a bitmask of combined modifiers.
type MouseModifier int

const (
	MouseNoModifier MouseModifier = 0
	MouseCtrl       MouseModifier = 0x01
	MouseShift      MouseModifier = 0x02
	MouseAlt        MouseModifier = 0x04
)

type MouseEvent struct {
	Action   MouseAction
	Buttons  MouseButtons
	Modifier MouseModifier
}

// EncodeMouse returns a first-layer click or wheel binding.
// Clicks accept non-empty combinations of left, right, and middle buttons.
// Source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/k8890.rs
func EncodeMouse(profile Profile, control Control, event MouseEvent) ([]Report, error) {
	slot, err := profile.slot(control)
	if err != nil {
		return nil, err
	}
	switch event.Modifier {
	case MouseNoModifier, MouseCtrl, MouseShift, MouseAlt:
	default:
		return nil, fmt.Errorf("unsupported TriSat mouse modifier %#x", event.Modifier)
	}
	var wheel byte
	switch event.Action {
	case MouseClick:
		if event.Buttons < 1 || event.Buttons > MouseLeft|MouseRight|MouseMiddle {
			return nil, fmt.Errorf("mouse click requires button bits 0x01..0x07")
		}
	case MouseWheelUp, MouseWheelDown:
		if event.Buttons != 0 {
			return nil, fmt.Errorf("mouse wheel binding cannot include buttons")
		}
		wheel = 0x01
		if event.Action == MouseWheelDown {
			wheel = 0xff
		}
	default:
		return nil, fmt.Errorf("unsupported TriSat mouse action %q", event.Action)
	}
	return bindingReports(Report{0x03, slot, 0x13, byte(event.Buttons & 255), 0, 0, wheel, byte(event.Modifier & 255)}), nil
}

// EncodeLED returns the separate LED sequence, not an A1 key-binding variant.
// Mode is a zero-based byte index. Named effects and hardware support are unverified.
// Sources: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/k8890.rs
// and https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/options.rs
func EncodeLED(profile Profile, mode int) ([]Report, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if mode < 0 || mode > 255 {
		return nil, fmt.Errorf("LED mode requires an index 0..255")
	}
	return []Report{
		{0x03, 0xa1, 0x01},
		{0x03, 0xb0, 0x18, byte(mode & 255)},
		{0x03, 0xaa, 0xa1},
	}, nil
}
