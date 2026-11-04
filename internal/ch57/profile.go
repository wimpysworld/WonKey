// Package ch57 encodes offline reports for an explicitly selected device profile.
// CH57 is a working family label, not a verified USB manufacturer identity.
// The reports have no hardware validation and do not authorise live writes.
package ch57

import "fmt"

type Variant string

const KriomantFE Variant = "kriomant-88aaa564-fe"

type Control string

const (
	Key1       Control = "key1"
	Key2       Control = "key2"
	Key3       Control = "key3"
	Key4       Control = "key4"
	Key5       Control = "key5"
	Key6       Control = "key6"
	Key7       Control = "key7"
	Key8       Control = "key8"
	Key9       Control = "key9"
	Key10      Control = "key10"
	Key11      Control = "key11"
	Key12      Control = "key12"
	KnobLeft   Control = "knob-left"
	KnobPress  Control = "knob-press"
	KnobRight  Control = "knob-right"
	Knob1Left  Control = "knob1-left"
	Knob1Press Control = "knob1-press"
	Knob1Right Control = "knob1-right"
	Knob2Left  Control = "knob2-left"
	Knob2Press Control = "knob2-press"
	Knob2Right Control = "knob2-right"
	Knob3Left  Control = "knob3-left"
	Knob3Press Control = "knob3-press"
	Knob3Right Control = "knob3-right"
)

var k8890CanonicalSlots = map[Control]byte{
	Key1:       1,
	Key2:       2,
	Key3:       3,
	Key4:       4,
	Key5:       5,
	Key6:       6,
	Key7:       7,
	Key8:       8,
	Key9:       9,
	Key10:      10,
	Key11:      11,
	Key12:      12,
	KnobLeft:   13,
	KnobPress:  14,
	KnobRight:  15,
	Knob1Left:  13,
	Knob1Press: 14,
	Knob1Right: 15,
	Knob2Left:  16,
	Knob2Press: 17,
	Knob2Right: 18,
	Knob3Left:  19,
	Knob3Press: 20,
	Knob3Right: 21,
}

type Profile struct {
	VID          uint16
	PID          uint16
	Variant      Variant
	Layer        int
	ControlSlots map[Control]byte
}

// ThreeKeyOneKnob returns an independent 1189:8890 profile.
// This profile does not establish compatibility with other product IDs or key counts.
// Slot source: https://github.com/kriomant/ch57x-keyboard-tool/blob/88aaa564/src/keyboard/mod.rs#L72-L80
func ThreeKeyOneKnob() Profile {
	return Profile{
		VID:     0x1189,
		PID:     0x8890,
		Variant: KriomantFE,
		ControlSlots: map[Control]byte{
			Key1:      1,
			Key2:      2,
			Key3:      3,
			KnobLeft:  13,
			KnobPress: 14,
			KnobRight: 15,
		},
	}
}

func (p Profile) slotAndLayer(control Control) (byte, byte, error) {
	if err := p.validate(); err != nil {
		return 0, 0, err
	}
	slot, ok := p.ControlSlots[control]
	if !ok {
		if alias, hasAlias := knob1Alias(control); hasAlias {
			slot, ok = p.ControlSlots[alias]
		}
	}
	if !ok {
		return 0, 0, fmt.Errorf("unsupported CH57 control %q", control)
	}
	layer := byte(p.Layer & 0x0f)
	if layer == 0 {
		layer = 1
	}
	return slot, layer, nil
}

func knob1Alias(control Control) (Control, bool) {
	switch control {
	case KnobLeft:
		return Knob1Left, true
	case KnobPress:
		return Knob1Press, true
	case KnobRight:
		return Knob1Right, true
	case Knob1Left:
		return KnobLeft, true
	case Knob1Press:
		return KnobPress, true
	case Knob1Right:
		return KnobRight, true
	default:
		return "", false
	}
}

func (p Profile) hasLimitedMultiStrokeModifiers() bool {
	if len(p.ControlSlots) != 6 {
		return false
	}
	for _, slot := range p.ControlSlots {
		if (slot < 1 || slot > 3) && (slot < 13 || slot > 15) {
			return false
		}
	}
	return true
}

func (p Profile) validate() error {
	if p.Variant != KriomantFE {
		return fmt.Errorf("unsupported CH57 variant %q", p.Variant)
	}
	if p.VID != 0x1189 || p.PID != 0x8890 || !validK8890ControlSlots(p.ControlSlots) {
		return fmt.Errorf("unsupported CH57 profile %04x:%04x or control slots", p.VID, p.PID)
	}
	if p.Layer < 0 || p.Layer > 15 {
		return fmt.Errorf("unsupported CH57 layer %d", p.Layer)
	}
	return nil
}

func validK8890ControlSlots(slots map[Control]byte) bool {
	if len(slots) == 0 {
		return false
	}
	var seen [22]bool
	for control, slot := range slots {
		canonical, ok := k8890CanonicalSlots[control]
		if !ok || slot != canonical || seen[slot] {
			return false
		}
		seen[slot] = true
	}
	return contiguousButtons(seen) && contiguousKnobs(seen)
}

func contiguousButtons(seen [22]bool) bool {
	missing := false
	for slot := 1; slot <= 12; slot++ {
		if !seen[slot] {
			missing = true
			continue
		}
		if missing {
			return false
		}
	}
	return true
}

func contiguousKnobs(seen [22]bool) bool {
	missing := false
	for base := 13; base <= 19; base += 3 {
		hasLeft, hasPress, hasRight := seen[base], seen[base+1], seen[base+2]
		if hasLeft != hasPress || hasPress != hasRight {
			return false
		}
		if !hasLeft {
			missing = true
			continue
		}
		if missing {
			return false
		}
	}
	return true
}
