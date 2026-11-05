package ch57

import "fmt"

// MediaUsage lists the consumer usages named in the pinned source.
// Source: https://github.com/kriomant/ch57x-keyboard-tool/blob/bdbffca3/src/keyboard/mod.rs
type MediaUsage int

const (
	MediaScreenBrightnessUp   MediaUsage = 0x6f
	MediaScreenBrightnessDown MediaUsage = 0x70
	MediaNext                 MediaUsage = 0xb5
	MediaPrevious             MediaUsage = 0xb6
	MediaStop                 MediaUsage = 0xb7
	MediaPlay                 MediaUsage = 0xcd
	MediaMute                 MediaUsage = 0xe2
	MediaVolumeUp             MediaUsage = 0xe9
	MediaVolumeDown           MediaUsage = 0xea
	MediaFavorites            MediaUsage = 0x182
	MediaCalculator           MediaUsage = 0x192
	MediaScreenLock           MediaUsage = 0x19e
	MediaWebPageHome          MediaUsage = 0x223
	MediaWebPageBack          MediaUsage = 0x224
	MediaWebPageForward       MediaUsage = 0x225
)

// EncodeMedia returns a layer binding with a 16-bit little-endian usage.
// Encoding source: https://github.com/kriomant/ch57x-keyboard-tool/blob/bdbffca3/src/keyboard/k8890.rs
func EncodeMedia(profile Profile, control Control, usage MediaUsage) ([]Report, error) {
	slot, layer, err := profile.slotAndLayer(control)
	if err != nil {
		return nil, err
	}
	if profile.Variant == SayoV2 {
		muCode, ok := SayoMediaCode(usage)
		if !ok {
			return nil, fmt.Errorf("unsupported CH57 media usage %#x", usage)
		}
		return sayoKeyReports(profile, slot, layer, [8]byte{0x03, 0x00, 0x00, 0x00, muCode, 0x00, 0x00, 0x00}), nil
	}
	switch usage {
	case MediaScreenBrightnessUp, MediaScreenBrightnessDown,
		MediaNext, MediaPrevious, MediaStop, MediaPlay, MediaMute,
		MediaVolumeUp, MediaVolumeDown, MediaFavorites, MediaCalculator, MediaScreenLock,
		MediaWebPageHome, MediaWebPageBack, MediaWebPageForward:
	default:
		return nil, fmt.Errorf("unsupported CH57 media usage %#x", usage)
	}
	switch profile.Variant {
	case K884xFE:
		return k884xFEReports(Report{0x03, 0xfe, slot, layer, 0x02, 0, 0, 0, 0, 0, 0x00, byte(usage & 255), byte((usage >> 8) & 255)}), nil
	case K884xFD:
		return k884xFDReports(Report{0x03, 0xfd, slot, layer, 0x02, 0x00, 0x02, 0x00, 0x00, byte(usage & 255), 0x00, 0x00, byte((usage >> 8) & 255)}), nil
	}
	return bindingReports(layer, Report{0x03, slot, (layer << 4) | 0x02, byte(usage & 255), byte((usage >> 8) & 255)}), nil
}

// SayoMediaCode maps a HID Consumer usage to the SayoDevice v2 multimedia code.
func SayoMediaCode(usage MediaUsage) (byte, bool) {
	switch usage {
	case MediaCalculator:
		return 1, true
	case MediaWebPageHome:
		return 5, true
	case MediaMute:
		return 8, true
	case MediaVolumeUp:
		return 10, true
	case MediaVolumeDown:
		return 11, true
	case MediaPlay, 0x00b0:
		return 12, true
	case MediaStop:
		return 13, true
	case MediaPrevious:
		return 14, true
	case MediaNext:
		return 15, true
	case MediaWebPageBack:
		return 19, true
	case MediaWebPageForward:
		return 20, true
	default:
		return 0, false
	}
}

// SayoMediaUsage maps a SayoDevice v2 multimedia code to a HID Consumer usage.
func SayoMediaUsage(code byte) uint16 {
	switch code {
	case 1:
		return uint16(MediaCalculator)
	case 5:
		return uint16(MediaWebPageHome)
	case 8:
		return uint16(MediaMute)
	case 10:
		return uint16(MediaVolumeUp)
	case 11:
		return uint16(MediaVolumeDown)
	case 12:
		return uint16(MediaPlay)
	case 13:
		return uint16(MediaStop)
	case 14:
		return uint16(MediaPrevious)
	case 15:
		return uint16(MediaNext)
	case 19:
		return uint16(MediaWebPageBack)
	case 20:
		return uint16(MediaWebPageForward)
	default:
		return uint16(code)
	}
}

type MouseAction string

const (
	MouseMove      MouseAction = "move"
	MouseDrag      MouseAction = "drag"
	MouseClick     MouseAction = "click"
	MouseWheel     MouseAction = "wheel"
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
	DX       int
	DY       int
	Wheel    int
	Modifier MouseModifier
}

// EncodeMouse returns a layer move, drag, click, or wheel binding.
// Clicks and drags accept non-empty combinations of left, right, and middle buttons.
// Source: https://github.com/kriomant/ch57x-keyboard-tool/blob/bdbffca3/src/keyboard/k8890.rs
func EncodeMouse(profile Profile, control Control, event MouseEvent) ([]Report, error) {
	slot, layer, err := profile.slotAndLayer(control)
	if err != nil {
		return nil, err
	}
	switch event.Modifier {
	case MouseNoModifier, MouseCtrl, MouseShift, MouseAlt:
	default:
		return nil, fmt.Errorf("unsupported CH57 mouse modifier %#x", event.Modifier)
	}
	buttons, dx, dy, wheel, err := encodeMousePayload(event)
	if err != nil {
		return nil, err
	}
	switch profile.Variant {
	case SayoV2:
		if event.Modifier != MouseNoModifier {
			return nil, fmt.Errorf("unsupported SayoDevice mouse modifier %#x", event.Modifier)
		}
		return sayoKeyReports(profile, slot, layer, [8]byte{0x02, 0x00, 0x00, 0x00, buttons, dx, dy, wheel}), nil
	case K884xFE:
		sub := byte(0x01)
		if wheel != 0 {
			sub = 0x03
		} else if dx != 0 || dy != 0 {
			sub = 0x05
		}
		return k884xFEReports(Report{0x03, 0xfe, slot, layer, 0x03, 0, 0, 0, 0, 0, sub, byte(event.Modifier & 255), buttons, dx, dy, wheel}), nil
	case K884xFD:
		var modByte byte
		switch event.Modifier {
		case MouseCtrl:
			modByte = 0xf1
		case MouseShift:
			modByte = 0xf2
		case MouseAlt:
			modByte = 0xf3
		}
		return k884xFDReports(Report{0x03, 0xfd, slot, layer, 0x03, 0x01, 0x04, 0x00, 0x00, modByte, 0x00, 0x00, buttons, 0x00, 0x00, dx, 0x00, 0x00, dy, 0x00, 0x00, wheel}), nil
	}
	return bindingReports(layer, Report{0x03, slot, (layer << 4) | 0x03, buttons, dx, dy, wheel, byte(event.Modifier & 255)}), nil
}

func encodeMousePayload(event MouseEvent) (byte, byte, byte, byte, error) {
	switch event.Action {
	case MouseClick:
		if !validMouseClick(event) {
			return 0, 0, 0, 0, fmt.Errorf("mouse click requires button bits 0x01..0x07 and zero deltas")
		}
		return byte(event.Buttons & 255), 0, 0, 0, nil
	case MouseMove:
		if !validMouseMove(event) {
			return 0, 0, 0, 0, fmt.Errorf("mouse move requires non-zero DX/DY in -128..127 and no buttons or wheel")
		}
		return 0, byte(event.DX & 255), byte(event.DY & 255), 0, nil
	case MouseDrag:
		if !validMouseDrag(event) {
			return 0, 0, 0, 0, fmt.Errorf("mouse drag requires button bits 0x01..0x07, non-zero DX/DY in -128..127, and no wheel")
		}
		return byte(event.Buttons & 255), byte(event.DX & 255), byte(event.DY & 255), 0, nil
	case MouseWheel:
		if !validMouseWheel(event) {
			return 0, 0, 0, 0, fmt.Errorf("mouse wheel requires non-zero wheel delta in -128..127 and no buttons or movement")
		}
		return 0, 0, 0, byte(event.Wheel & 255), nil
	case MouseWheelUp, MouseWheelDown:
		if !validMouseWheelDirection(event) {
			return 0, 0, 0, 0, fmt.Errorf("mouse wheel direction binding cannot include buttons, movement, or wheel delta")
		}
		if event.Action == MouseWheelDown {
			return 0, 0, 0, 0xff, nil
		}
		return 0, 0, 0, 0x01, nil
	default:
		return 0, 0, 0, 0, fmt.Errorf("unsupported CH57 mouse action %q", event.Action)
	}
}

func validMouseClick(event MouseEvent) bool {
	return validMouseButtons(event.Buttons) && event.DX == 0 && event.DY == 0 && event.Wheel == 0
}

func validMouseMove(event MouseEvent) bool {
	return event.Buttons == 0 && event.Wheel == 0 && validMouseDeltaPair(event.DX, event.DY)
}

func validMouseDrag(event MouseEvent) bool {
	return validMouseButtons(event.Buttons) && event.Wheel == 0 && validMouseDeltaPair(event.DX, event.DY)
}

func validMouseWheel(event MouseEvent) bool {
	return event.Buttons == 0 && event.DX == 0 && event.DY == 0 && validSignedByte(event.Wheel) && event.Wheel != 0
}

func validMouseWheelDirection(event MouseEvent) bool {
	return event.Buttons == 0 && event.DX == 0 && event.DY == 0 && event.Wheel == 0
}

func validMouseButtons(buttons MouseButtons) bool {
	return buttons >= 1 && buttons <= MouseLeft|MouseRight|MouseMiddle
}

func validMouseDeltaPair(dx, dy int) bool {
	return validSignedByte(dx) && validSignedByte(dy) && (dx != 0 || dy != 0)
}

func validSignedByte(value int) bool {
	return value >= -128 && value <= 127
}

// EncodeLED returns the separate LED sequence, not an A1 key-binding variant.
// Mode is a zero-based byte index. Named effects and hardware support are unverified.
// Sources: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/k8890.rs
// and https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/options.rs
func EncodeLED(profile Profile, mode int) ([]Report, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if profile.Variant == SayoV2 {
		return nil, fmt.Errorf("SayoDevice profile does not support LED mode; use EncodeSayoRGB")
	}
	if mode < 0 || mode > 255 {
		return nil, fmt.Errorf("LED mode requires an index 0..255")
	}
	switch profile.Variant {
	case K884xFE:
		return k884xFDReports(Report{0x03, 0xfe, 0xb0, 0x01, 0x08, 0, 0, 0, 0, 0, 0x01, 0x00, byte(mode & 255)}), nil
	case K884xFD:
		return k884xFDReports(Report{0x03, 0xfe, 0xb0, 0x00, byte(mode & 255)}), nil
	}
	return []Report{
		{0x03, 0xa1, 0x01},
		{0x03, 0xb0, 0x18, byte(mode & 255)},
		{0x03, 0xaa, 0xa1},
	}, nil
}

// EncodeSayoRGB returns the SayoDevice v2 per-key Light (0x11) and Save (0x0D) reports.
func EncodeSayoRGB(profile Profile, mode string, r, g, b byte, hasColour bool) ([]Report, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if profile.Variant != SayoV2 {
		return nil, fmt.Errorf("profile %q does not support SayoDevice RGB", profile.Variant)
	}
	var ledMode, ledEvent byte
	preserveColour := !hasColour
	switch mode {
	case "static":
		ledMode, ledEvent = 0x00, 0x00
	case "held":
		ledMode, ledEvent = 0x00, 0x01
	case "breathe":
		ledMode, ledEvent = 0x02, 0x00
	case "cycle-slow", "cycle-fast":
		ledMode, ledEvent = 0x04, 0x00
	case "flash":
		ledMode, ledEvent = 0x08, 0x01
	case "off":
		ledMode, ledEvent = 0x00, 0x00
		r, g, b = 0, 0, 0
		preserveColour = false
	default:
		return nil, fmt.Errorf("unsupported SayoDevice RGB mode %q", mode)
	}
	keyCount := len(profile.ControlSlots)
	reports := make([]Report, 0, keyCount+1)
	for index := range keyCount {
		pkt := Report{0x21, 0x12, 0x00, 0x00, 0x3c, 0x00, 0x11, byte(index)}
		if preserveColour {
			pkt[8] = 1
		}
		pkt[24] = ledMode
		pkt[25] = ledEvent
		pkt[26] = 0x64
		pkt[27] = 0x64
		pkt[28] = r
		pkt[29] = g
		pkt[30] = b
		pkt[31] = 0x00
		SayoSignPacket(&pkt)
		reports = append(reports, pkt)
	}
	return append(reports, SayoSaveReport()), nil
}
