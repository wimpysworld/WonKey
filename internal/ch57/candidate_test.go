package ch57

import (
	"bytes"
	"strings"
	"testing"
)

// syntheticCandidateDescriptors reconstructs the published fields independently.
// These bytes are not a capture from the user's device.
func syntheticCandidateDescriptors() []byte {
	descriptors := []byte{
		18, 1, 0x10, 0x01, 0, 0, 0, 8, 0x89, 0x11, 0x90, 0x88, 0, 0, 0, 0, 0, 1,
		9, 2, 109, 0, 4, 1, 0, 0xa0, 50,
	}
	for i, fields := range [4]struct {
		subclass byte
		protocol byte
		hid      uint16
		report   byte
		endpoint byte
		packet   byte
	}{
		{1, 1, 0x0111, 64, 0x81, 64},
		{0, 0, 0x0100, 36, 0x02, 64},
		{0, 0, 0x0100, 86, 0x83, 4},
		{1, 2, 0x0110, 52, 0x82, 4},
	} {
		descriptors = append(descriptors,
			9, 4, byte(i), 0, 1, 3, fields.subclass, fields.protocol, 0,
			9, 0x21, byte(fields.hid&0xff), byte(fields.hid>>8), 0, 1, 0x22, fields.report, 0,
			7, 5, fields.endpoint, 3, fields.packet, 0, 10,
		)
	}
	return descriptors
}

func TestMatchCandidate(t *testing.T) {
	descriptors := syntheticCandidateDescriptors()
	original := bytes.Clone(descriptors)
	got, err := MatchCandidate("1-2.3", descriptors)
	if err != nil {
		t.Fatal(err)
	}
	want := Candidate{
		PhysicalPath:  "1-2.3",
		VID:           0x1189,
		PID:           0x8890,
		Revision:      0,
		Configuration: 1,
		Interfaces: [4]CandidateInterface{
			{0, 0, 3, 1, 1, 0x0111, 64, CandidateEndpoint{0x81, 3, 64, 10}},
			{1, 0, 3, 0, 0, 0x0100, 36, CandidateEndpoint{0x02, 3, 64, 10}},
			{2, 0, 3, 0, 0, 0x0100, 86, CandidateEndpoint{0x83, 3, 4, 10}},
			{3, 0, 3, 1, 2, 0x0110, 52, CandidateEndpoint{0x82, 3, 4, 10}},
		},
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !bytes.Equal(descriptors, original) {
		t.Fatal("matcher changed the input")
	}
}

func TestMatchCandidateRejectsUnknownStructure(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func([]byte) []byte
	}{
		{"wrong vendor", func(d []byte) []byte { d[8]++; return d }},
		{"wrong product", func(d []byte) []byte { d[10]++; return d }},
		{"unknown revision", func(d []byte) []byte { d[12]++; return d }},
		{"multiple configurations", func(d []byte) []byte { d[17] = 2; return d }},
		{"wrong total length", func(d []byte) []byte { d[20]--; return d }},
		{"unknown interface count", func(d []byte) []byte { d[22] = 3; return d }},
		{"duplicate interface", func(d []byte) []byte { d[54] = 0; return d }},
		{"alternate setting", func(d []byte) []byte { d[55] = 1; return d }},
		{"extra vendor endpoint", func(d []byte) []byte { d[56] = 2; return d }},
		{"unknown HID class", func(d []byte) []byte { d[57] = 0xff; return d }},
		{"missing report", func(d []byte) []byte { d[68] = 0; return d }},
		{"vendor IN instead of OUT", func(d []byte) []byte { d[72] = 0x82; return d }},
		{"non-interrupt endpoint", func(d []byte) []byte { d[73] = 2; return d }},
		{"unknown packet size", func(d []byte) []byte { d[74] = 8; return d }},
		{"zero length descriptor", func(d []byte) []byte { d[52] = 0; return d }},
		{"short descriptor", func(d []byte) []byte { d[52] = 1; return d }},
		{"oversized descriptor", func(d []byte) []byte { d[52] = 255; return d }},
		{"unknown descriptor type", func(d []byte) []byte { d[53] = 0xff; return d }},
		{"duplicate device", func(d []byte) []byte { return append(d, d[:18]...) }},
		{"duplicate configuration", func(d []byte) []byte { return append(d, d[18:]...) }},
		{"extra descriptor", func(d []byte) []byte {
			d = append(d, 2, 0xff)
			d[20] += 2
			return d
		}},
		{"trailing garbage", func(d []byte) []byte { return append(d, 0) }},
		{"oversized input", func(d []byte) []byte { return bytes.Repeat(d, 1024) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchCandidate("1-2", tt.change(syntheticCandidateDescriptors()))
			if err == nil || got != (Candidate{}) {
				t.Fatalf("got %+v, %v, want no candidate and an error", got, err)
			}
		})
	}
}

func TestMatchCandidateRejectsEveryTruncationAndByteChange(t *testing.T) {
	d := syntheticCandidateDescriptors()
	for length := range len(d) {
		if got, err := MatchCandidate("1-2", d[:length]); err == nil || got != (Candidate{}) {
			t.Fatalf("accepted truncation to %d bytes", length)
		}
	}
	for offset, original := range d {
		for value := range 256 {
			if byte(value) == original {
				continue
			}
			d[offset] = byte(value)
			if got, err := MatchCandidate("1-2", d); err == nil || got != (Candidate{}) {
				t.Fatalf("accepted byte %d changed to %02x", offset, value)
			}
		}
		d[offset] = original
	}
}

func TestMatchCandidatePhysicalPath(t *testing.T) {
	descriptors := syntheticCandidateDescriptors()
	for _, path := range []string{"1-2", "12-10.3.4", "2-1.1.1.1.1.1"} {
		got, err := MatchCandidate(path, descriptors)
		if err != nil || got.PhysicalPath != path {
			t.Fatalf("path %q: got %+v, %v", path, got, err)
		}
	}
	for _, path := range []string{
		"", ".", "..", "../1-2", "/sys/bus/usb/devices/1-2", "1-2/", "1-2:1.1",
		"hidraw0", "usb1", "0-1", "1-0", "01-2", "1-02", "1-2.0", "1-2.",
		"1-2\n", "1-2\x00", strings.Repeat("1", 254) + "-2",
	} {
		got, err := MatchCandidate(path, descriptors)
		if err == nil || got != (Candidate{}) {
			t.Fatalf("accepted invalid path %q: %+v, %v", path, got, err)
		}
	}
}

func TestMatchCandidate514C(t *testing.T) {
	for _, tc := range []struct {
		name        string
		descriptors []byte
		pid         uint16
		ep0, ep1    byte
	}{
		{"514c:8851", []byte(candidate8851USBDescriptor), 0x8851, 0x82, 0x81},
		{"514c:8850", []byte(candidate8850USBDescriptor), 0x8850, 0x84, 0x82},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MatchCandidate("1-4.1", tc.descriptors)
			if err != nil {
				t.Fatal(err)
			}
			want := Candidate{
				PhysicalPath:  "1-4.1",
				VID:           0x514c,
				PID:           tc.pid,
				Revision:      0x0100,
				Configuration: 1,
				Interfaces: [4]CandidateInterface{
					{0, 0, 3, 0, 0, 0x0111, 36, CandidateEndpoint{tc.ep0, 3, 64, 1}},
					{1, 0, 3, 1, 1, 0x0201, 207, CandidateEndpoint{tc.ep1, 3, 16, 1}},
				},
			}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestMatchCandidateSayo000C(t *testing.T) {
	got, err := MatchCandidate("7-1.4.3.4", []byte(candidateSayo000CUSBDescriptor))
	if err != nil {
		t.Fatal(err)
	}
	want := Candidate{
		PhysicalPath:  "7-1.4.3.4",
		VID:           0x8089,
		PID:           0x000c,
		Revision:      0x089a,
		Configuration: 1,
		Interfaces: [4]CandidateInterface{
			{0, 0, 3, 1, 1, 0x0110, 65, CandidateEndpoint{0x81, 3, 8, 1}},
			{1, 0, 3, 0, 0, 0x0111, 154, CandidateEndpoint{0x82, 3, 64, 1}},
		},
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func FuzzMatchCandidate(f *testing.F) {
	f.Add("1-2", syntheticCandidateDescriptors())
	f.Add("1-2", []byte(candidate8851USBDescriptor))
	f.Add("1-2", []byte(candidate8850USBDescriptor))
	f.Add("7-1.4.3.4", []byte(candidateSayo000CUSBDescriptor))
	f.Add("1-2", []byte{})
	f.Add("hidraw0", syntheticCandidateDescriptors())
	f.Fuzz(func(t *testing.T, path string, descriptors []byte) {
		got, err := MatchCandidate(path, descriptors)
		if err != nil {
			if got != (Candidate{}) {
				t.Fatal("rejection returned a candidate")
			}
			return
		}
		valid := bytes.Equal(descriptors, syntheticCandidateDescriptors()) ||
			bytes.Equal(descriptors, []byte(candidate8851USBDescriptor)) ||
			bytes.Equal(descriptors, []byte(candidate8850USBDescriptor)) ||
			bytes.Equal(descriptors, []byte(candidateSayo000CUSBDescriptor))
		if !valid || got.PhysicalPath != path {
			t.Fatal("accepted an unknown structure or changed the physical path")
		}
	})
}
