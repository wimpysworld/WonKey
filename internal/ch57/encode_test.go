package ch57

import (
	"maps"
	"slices"
	"testing"
)

func TestSingleStrokeAllControls(t *testing.T) {
	for _, tt := range []struct {
		control Control
		slot    byte
	}{
		{Key1, 1},
		{Key2, 2},
		{Key3, 3},
		{KnobLeft, 13},
		{KnobPress, 14},
		{KnobRight, 15},
	} {
		t.Run(string(tt.control), func(t *testing.T) {
			got, err := EncodeKeyboard(ThreeKeyOneKnob(), tt.control, []Stroke{{Modifiers: 0x02, Usage: 0x04}})
			if err != nil {
				t.Fatal(err)
			}
			want := []Report{
				{0x03, 0xfe, 0x01, 0x01, 0x01},
				{0x03, tt.slot, 0x11, 0x01, 0x00, 0x00, 0x00},
				{0x03, tt.slot, 0x11, 0x01, 0x01, 0x02, 0x04},
				{0x03, 0xaa, 0xaa},
			}
			assertReports(t, got, want)
		})
	}
}

func TestFiveStrokes(t *testing.T) {
	strokes := []Stroke{
		{Modifiers: 0x01, Usage: 0x04},
		{Modifiers: 0x02, Usage: 0x05},
		{Modifiers: 0x04, Usage: 0x28},
		{Modifiers: 0x80, Usage: 0x68},
		{Modifiers: 0xff, Usage: 0x73},
	}
	got, err := EncodeKeyboard(ThreeKeyOneKnob(), Key2, strokes)
	if err != nil {
		t.Fatal(err)
	}
	want := []Report{
		{0x03, 0xfe, 0x01, 0x01, 0x01},
		{0x03, 0x02, 0x11, 0x05, 0x00, 0x00, 0x00},
		{0x03, 0x02, 0x11, 0x05, 0x01, 0x01, 0x04},
		{0x03, 0x02, 0x11, 0x05, 0x02, 0x02, 0x05},
		{0x03, 0x02, 0x11, 0x05, 0x03, 0x04, 0x28},
		{0x03, 0x02, 0x11, 0x05, 0x04, 0x80, 0x68},
		{0x03, 0x02, 0x11, 0x05, 0x05, 0xff, 0x73},
		{0x03, 0xaa, 0xaa},
	}
	assertReports(t, got, want)
}

func TestStrokeCounts(t *testing.T) {
	for count := 1; count <= 5; count++ {
		strokes := slices.Repeat([]Stroke{{Usage: 0x04}}, count)
		got, err := EncodeKeyboard(ThreeKeyOneKnob(), Key1, strokes)
		if err != nil {
			t.Fatal(count, err)
		}
		want := []Report{{0x03, 0xfe, 0x01, 0x01, 0x01}, {0x03, 1, 0x11, byte(count)}}
		for index := 1; index <= count; index++ {
			want = append(want, Report{0x03, 1, 0x11, byte(count), byte(index), 0, 0x04})
		}
		want = append(want, Report{0x03, 0xaa, 0xaa})
		assertReports(t, got, want)
	}
}

func TestRejectInvalidStrokes(t *testing.T) {
	for _, tt := range []struct {
		name    string
		strokes []Stroke
	}{
		{"nil", nil},
		{"empty", []Stroke{}},
		{"too many", slices.Repeat([]Stroke{{Usage: 0x04}}, 6)},
		{"negative modifiers", []Stroke{{Modifiers: -1, Usage: 0x04}}},
		{"large modifiers", []Stroke{{Modifiers: 256, Usage: 0x04}}},
		{"negative usage", []Stroke{{Usage: -1}}},
		{"empty usage", []Stroke{{Usage: 0}}},
		{"error usage", []Stroke{{Usage: 0x03}}},
		{"unsupported usage", []Stroke{{Usage: 0x74}}},
		{"modifier usage", []Stroke{{Usage: 0xe0}}},
		{"large usage", []Stroke{{Usage: 256}}},
		{"invalid final stroke", []Stroke{{Usage: 0x04}, {Usage: 0x03}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeKeyboard(ThreeKeyOneKnob(), Key1, tt.strokes)
			if err == nil || got != nil {
				t.Fatalf("got %x, %v, want nil reports and an error", got, err)
			}
		})
	}
}

func TestModifierAndUsageBoundaries(t *testing.T) {
	for modifiers := 0; modifiers <= 255; modifiers++ {
		for _, usage := range []int{0x04, 0x73} {
			got, err := EncodeKeyboard(ThreeKeyOneKnob(), Key1, []Stroke{{Modifiers: modifiers, Usage: usage}})
			if err != nil {
				t.Fatal(modifiers, usage, err)
			}
			if int(got[2][5]) != modifiers || int(got[2][6]) != usage {
				t.Fatalf("stroke changed: %x", got[2])
			}
		}
	}
}

func TestRejectInvalidProfilesAndControls(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Profile)
	}{
		{"zero profile", func(p *Profile) { *p = Profile{} }},
		{"vendor", func(p *Profile) { p.VID = 0x8844 }},
		{"product", func(p *Profile) { p.PID = 0x8840 }},
		{"empty variant", func(p *Profile) { p.Variant = "" }},
		{"A1 variant", func(p *Profile) { p.Variant = "a1" }},
		{"unknown variant", func(p *Profile) { p.Variant = "other" }},
		{"nil controls", func(p *Profile) { p.ControlSlots = nil }},
		{"missing control", func(p *Profile) { delete(p.ControlSlots, Key2) }},
		{"extra control", func(p *Profile) { p.ControlSlots["key4"] = 4 }},
		{"remapped control", func(p *Profile) { p.ControlSlots[KnobLeft] = 4 }},
		{"duplicate slot", func(p *Profile) { p.ControlSlots[Key2] = 1 }},
		{"zero slot", func(p *Profile) { p.ControlSlots[Key1] = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			profile := ThreeKeyOneKnob()
			tt.change(&profile)
			encoders := bindingEncoders()
			encoders["LED"] = func(p Profile, _ Control) ([]Report, error) { return EncodeLED(p, 0) }
			for name, encode := range encoders {
				got, err := encode(profile, Key1)
				if err == nil || got != nil {
					t.Fatalf("%s: got %x, %v, want nil reports and an error", name, got, err)
				}
			}
		})
	}
	for _, control := range []Control{"", "key4", "13", "knob2-left"} {
		for name, encode := range bindingEncoders() {
			got, err := encode(ThreeKeyOneKnob(), control)
			if err == nil || got != nil {
				t.Fatalf("%s control %q: got %x, %v", name, control, got, err)
			}
		}
	}
}

func TestEncoderHasNoSharedState(t *testing.T) {
	profile := ThreeKeyOneKnob()
	if profile.VID != 0x1189 || profile.PID != 0x8890 || profile.Variant != KriomantFE {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	originalSlots := maps.Clone(profile.ControlSlots)
	strokes := []Stroke{{Modifiers: 0x40, Usage: 0x04}}
	originalStrokes := slices.Clone(strokes)
	first, err := EncodeKeyboard(profile, KnobPress, strokes)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(profile.ControlSlots, originalSlots) || !slices.Equal(strokes, originalStrokes) {
		t.Fatal("encoder changed its inputs")
	}
	want := slices.Clone(first)
	first[0][0] = 0xff
	second, err := EncodeKeyboard(profile, KnobPress, strokes)
	if err != nil {
		t.Fatal(err)
	}
	assertReports(t, second, want)
	profile.ControlSlots[Key1] = 9
	if ThreeKeyOneKnob().ControlSlots[Key1] != 1 {
		t.Fatal("profiles share a mutable control map")
	}
	strokes[0].Usage = 0x05
	assertReports(t, second, want)
}

func assertReports(t *testing.T, got, want []Report) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("reports:\n got %x\nwant %x", got, want)
	}
	for i, report := range got {
		if len(report) != 64 || report[0] != 0x03 {
			t.Fatalf("report %d has invalid framing: %x", i, report)
		}
		for offset, value := range report[8:] {
			if value != 0 {
				t.Fatalf("report %d has non-zero padding at %d", i, offset+8)
			}
		}
	}
}
