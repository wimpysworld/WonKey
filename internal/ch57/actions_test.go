package ch57

import (
	"fmt"
	"maps"
	"slices"
	"testing"
)

func bindingEncoders() map[string]func(Profile, Control) ([]Report, error) {
	return map[string]func(Profile, Control) ([]Report, error){
		"keyboard": func(p Profile, c Control) ([]Report, error) {
			return EncodeKeyboard(p, c, []Stroke{{Usage: 0x04}})
		},
		"media": func(p Profile, c Control) ([]Report, error) { return EncodeMedia(p, c, MediaCalculator) },
		"click": func(p Profile, c Control) ([]Report, error) {
			return EncodeMouse(p, c, MouseEvent{Action: MouseClick, Buttons: MouseLeft, Modifier: MouseCtrl})
		},
		"wheel up": func(p Profile, c Control) ([]Report, error) {
			return EncodeMouse(p, c, MouseEvent{Action: MouseWheelUp, Modifier: MouseShift})
		},
		"wheel down": func(p Profile, c Control) ([]Report, error) {
			return EncodeMouse(p, c, MouseEvent{Action: MouseWheelDown, Modifier: MouseAlt})
		},
	}
}

func TestMediaGoldenAllControls(t *testing.T) {
	for _, control := range []struct {
		name Control
		slot byte
	}{{Key1, 1}, {Key2, 2}, {Key3, 3}, {KnobLeft, 13}, {KnobPress, 14}, {KnobRight, 15}} {
		for _, usage := range []struct {
			value MediaUsage
			low   byte
			high  byte
		}{
			{MediaNext, 0xb5, 0x00},
			{MediaPrevious, 0xb6, 0x00},
			{MediaStop, 0xb7, 0x00},
			{MediaPlay, 0xcd, 0x00},
			{MediaMute, 0xe2, 0x00},
			{MediaVolumeUp, 0xe9, 0x00},
			{MediaVolumeDown, 0xea, 0x00},
			{MediaFavorites, 0x82, 0x01},
			{MediaCalculator, 0x92, 0x01},
			{MediaScreenLock, 0x9e, 0x01},
		} {
			t.Run(fmt.Sprintf("%s/%x", control.name, usage.value), func(t *testing.T) {
				got, err := EncodeMedia(ThreeKeyOneKnob(), control.name, usage.value)
				if err != nil {
					t.Fatal(err)
				}
				assertReports(t, got, []Report{
					{0x03, 0xfe, 0x01, 0x01, 0x01},
					{0x03, control.slot, 0x12, usage.low, usage.high},
					{0x03, 0xaa, 0xaa},
				})
			})
		}
	}
}

func TestMouseGoldenAllControls(t *testing.T) {
	for _, control := range []struct {
		name Control
		slot byte
	}{{Key1, 1}, {Key2, 2}, {Key3, 3}, {KnobLeft, 13}, {KnobPress, 14}, {KnobRight, 15}} {
		for _, action := range []struct {
			event  MouseEvent
			button byte
			wheel  byte
		}{
			{MouseEvent{Action: MouseClick, Buttons: MouseLeft}, 0x01, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseRight}, 0x02, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseLeft | MouseRight}, 0x03, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseMiddle}, 0x04, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseLeft | MouseMiddle}, 0x05, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseRight | MouseMiddle}, 0x06, 0x00},
			{MouseEvent{Action: MouseClick, Buttons: MouseLeft | MouseRight | MouseMiddle}, 0x07, 0x00},
			{MouseEvent{Action: MouseWheelUp}, 0x00, 0x01},
			{MouseEvent{Action: MouseWheelDown}, 0x00, 0xff},
		} {
			for _, modifier := range []struct {
				value MouseModifier
				wire  byte
			}{{MouseNoModifier, 0x00}, {MouseCtrl, 0x01}, {MouseShift, 0x02}, {MouseAlt, 0x04}} {
				t.Run(fmt.Sprintf("%s/%s/%x/%x", control.name, action.event.Action, action.button, modifier.wire), func(t *testing.T) {
					event := action.event
					event.Modifier = modifier.value
					got, err := EncodeMouse(ThreeKeyOneKnob(), control.name, event)
					if err != nil {
						t.Fatal(err)
					}
					assertReports(t, got, []Report{
						{0x03, 0xfe, 0x01, 0x01, 0x01},
						{0x03, control.slot, 0x13, action.button, 0x00, 0x00, action.wheel, modifier.wire},
						{0x03, 0xaa, 0xaa},
					})
				})
			}
		}
	}
}

func TestLEDGolden(t *testing.T) {
	for _, tt := range []struct {
		mode int
		wire byte
	}{{0, 0x00}, {1, 0x01}, {127, 0x7f}, {255, 0xff}} {
		got, err := EncodeLED(ThreeKeyOneKnob(), tt.mode)
		if err != nil {
			t.Fatal(err)
		}
		assertReports(t, got, []Report{
			{0x03, 0xa1, 0x01},
			{0x03, 0xb0, 0x18, tt.wire},
			{0x03, 0xaa, 0xa1},
		})
	}
}

func TestRejectInvalidActions(t *testing.T) {
	for _, usage := range []MediaUsage{-1, 0, 0x04, 0xb4, 0xb8, 0x100, 0x181, 0x183, 0xffff, 0x10000, 0x100b5} {
		got, err := EncodeMedia(ThreeKeyOneKnob(), Key1, usage)
		if err == nil || got != nil {
			t.Fatalf("media %#x: got %x, %v", usage, got, err)
		}
	}
	for _, mode := range []int{-1, 256, 65536} {
		got, err := EncodeLED(ThreeKeyOneKnob(), mode)
		if err == nil || got != nil {
			t.Fatalf("LED %d: got %x, %v", mode, got, err)
		}
	}
	for _, event := range []MouseEvent{
		{},
		{Action: "move"},
		{Action: MouseClick},
		{Action: MouseClick, Buttons: -1},
		{Action: MouseClick, Buttons: 8},
		{Action: MouseClick, Buttons: 255},
		{Action: MouseClick, Buttons: 257},
		{Action: MouseWheelUp, Buttons: MouseLeft},
		{Action: MouseWheelDown, Buttons: MouseRight},
		{Action: MouseWheelDown, Buttons: -1},
		{Action: MouseWheelUp, Modifier: -1},
		{Action: MouseClick, Buttons: MouseLeft, Modifier: 3},
		{Action: MouseWheelUp, Modifier: 5},
		{Action: MouseWheelDown, Modifier: 6},
		{Action: MouseWheelDown, Modifier: 7},
		{Action: MouseWheelUp, Modifier: 8},
		{Action: MouseWheelUp, Modifier: 256},
		{Action: MouseWheelUp, Modifier: 257},
	} {
		got, err := EncodeMouse(ThreeKeyOneKnob(), Key1, event)
		if err == nil || got != nil {
			t.Fatalf("mouse %+v: got %x, %v", event, got, err)
		}
	}
}

func TestActionsHaveNoSharedState(t *testing.T) {
	encoders := bindingEncoders()
	encoders["LED"] = func(p Profile, _ Control) ([]Report, error) { return EncodeLED(p, 127) }
	for name, encode := range encoders {
		t.Run(name, func(t *testing.T) {
			profile := ThreeKeyOneKnob()
			slots := maps.Clone(profile.ControlSlots)
			first, err := encode(profile, KnobRight)
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(profile.ControlSlots, slots) {
				t.Fatal("encoder changed the control map")
			}
			want := slices.Clone(first)
			for i := range first {
				for j := range first[i] {
					first[i][j] ^= 0xff
				}
			}
			second, err := encode(profile, KnobRight)
			if err != nil {
				t.Fatal(err)
			}
			assertReports(t, second, want)
			profile.ControlSlots[KnobRight] = 4
			assertReports(t, second, want)
		})
	}
}
