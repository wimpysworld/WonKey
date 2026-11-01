// Package trisat encodes offline reports for an explicitly selected device profile.
// TriSat is a working family label, not a verified USB manufacturer identity.
// The reports have no hardware validation and do not authorise live writes.
package trisat

import (
	"fmt"
	"maps"
)

type Variant string

const KriomantFE Variant = "kriomant-88aaa564-fe"

type Control string

const (
	Key1      Control = "key1"
	Key2      Control = "key2"
	Key3      Control = "key3"
	KnobLeft  Control = "knob-left"
	KnobPress Control = "knob-press"
	KnobRight Control = "knob-right"
)

type Profile struct {
	VID          uint16
	PID          uint16
	Variant      Variant
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

func (p Profile) slot(control Control) (byte, error) {
	if err := p.validate(); err != nil {
		return 0, err
	}
	slot, ok := p.ControlSlots[control]
	if !ok {
		return 0, fmt.Errorf("unsupported TriSat control %q", control)
	}
	return slot, nil
}

func (p Profile) validate() error {
	if p.Variant != KriomantFE {
		return fmt.Errorf("unsupported TriSat variant %q", p.Variant)
	}
	known := ThreeKeyOneKnob()
	if p.VID != known.VID || p.PID != known.PID || !maps.Equal(p.ControlSlots, known.ControlSlots) {
		return fmt.Errorf("unsupported TriSat profile %04x:%04x or control slots", p.VID, p.PID)
	}
	return nil
}
