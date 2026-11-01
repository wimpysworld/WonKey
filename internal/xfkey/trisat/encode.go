package trisat

import "fmt"

// Report includes report ID 0x03 in its 64 bytes, without a transport prefix.
// Framing source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/mod.rs#L27-L36
type Report [64]byte

// Stroke uses the eight standard keyboard modifier bits and a keyboard HID usage.
// Supported usages are 0x04 (A) through 0x73 (F24), as named in the pinned source.
// Modifier-only strokes and raw custom usages are not supported.
type Stroke struct {
	Modifiers int
	Usage     int
}

// EncodeKeyboard returns a first-layer binding for one to five ordered strokes.
// It performs no device access and does not change the profile or strokes.
// Encoding source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/k8890.rs#L13-L79
func EncodeKeyboard(profile Profile, control Control, strokes []Stroke) ([]Report, error) {
	slot, err := profile.slot(control)
	if err != nil {
		return nil, err
	}
	count := len(strokes)
	if count < 1 || count > 5 {
		return nil, fmt.Errorf("keyboard binding requires 1..5 strokes")
	}
	for i, stroke := range strokes {
		if stroke.Modifiers < 0 || stroke.Modifiers > 255 || stroke.Usage < 0x04 || stroke.Usage > 0x73 {
			return nil, fmt.Errorf("stroke %d requires modifiers 0..255 and usage 0x04..0x73", i+1)
		}
	}

	records := make([]Report, 0, count+1)
	// The source inserts an empty record at index zero, outside the stroke count.
	records = append(records, Report{0x03, slot, 0x11, byte(count), 0, 0, 0})
	for i, stroke := range strokes {
		records = append(records, Report{0x03, slot, 0x11, byte(count), byte(i + 1), byte(stroke.Modifiers & 255), byte(stroke.Usage & 255)})
	}
	return bindingReports(records...), nil
}

func bindingReports(records ...Report) []Report {
	reports := make([]Report, 0, len(records)+2)
	reports = append(reports, Report{0x03, 0xfe, 0x01, 0x01, 0x01})
	reports = append(reports, records...)
	return append(reports, Report{0x03, 0xaa, 0xaa})
}
