package ch57

import (
	"encoding/binary"
	"fmt"
	"regexp"
)

// This is the USB descriptor structure published at
// https://github.com/the-via/keyboards/issues/2169, not a device capture.
// Exact matching deliberately excludes other 1189:8890 layouts and revisions.
const candidateUSBDescriptor = "\x12\x01\x10\x01\x00\x00\x00\x08\x89\x11\x90\x88\x00\x00\x00\x00\x00\x01" +
	"\x09\x02\x6d\x00\x04\x01\x00\xa0\x32" +
	"\x09\x04\x00\x00\x01\x03\x01\x01\x00\x09\x21\x11\x01\x00\x01\x22\x40\x00\x07\x05\x81\x03\x40\x00\x0a" +
	"\x09\x04\x01\x00\x01\x03\x00\x00\x00\x09\x21\x00\x01\x00\x01\x22\x24\x00\x07\x05\x02\x03\x40\x00\x0a" +
	"\x09\x04\x02\x00\x01\x03\x00\x00\x00\x09\x21\x00\x01\x00\x01\x22\x56\x00\x07\x05\x83\x03\x04\x00\x0a" +
	"\x09\x04\x03\x00\x01\x03\x01\x02\x00\x09\x21\x10\x01\x00\x01\x22\x34\x00\x07\x05\x82\x03\x04\x00\x0a"

const candidate8851USBDescriptor = "\x12\x01\x10\x01\x00\x00\x00\x40\x4c\x51\x51\x88\x00\x01\x01\x02\x03\x01" +
	"\x09\x02\x42\x00\x02\x01\x00\x80\xc8" +
	"\x09\x04\x00\x00\x02\x03\x00\x00\x00\x09\x21\x11\x01\x00\x01\x22\x24\x00\x07\x05\x82\x03\x40\x00\x01\x07\x05\x02\x03\x40\x00\x01" +
	"\x09\x04\x01\x00\x01\x03\x01\x01\x00\x09\x21\x01\x02\x00\x01\x22\xcf\x00\x07\x05\x81\x03\x10\x00\x01"

const candidate8850USBDescriptor = "\x12\x01\x10\x01\x00\x00\x00\x40\x4c\x51\x50\x88\x00\x01\x01\x02\x03\x01" +
	"\x09\x02\x42\x00\x02\x01\x00\xa0\x32" +
	"\x09\x04\x00\x00\x02\x03\x00\x00\x00\x09\x21\x11\x01\x00\x01\x22\x24\x00\x07\x05\x84\x03\x40\x00\x01\x07\x05\x04\x03\x40\x00\x01" +
	"\x09\x04\x01\x00\x01\x03\x01\x01\x00\x09\x21\x01\x02\x00\x01\x22\xcf\x00\x07\x05\x82\x03\x10\x00\x01"

var physicalUSBPath = regexp.MustCompile(`^[1-9][0-9]*-[1-9][0-9]*(\.[1-9][0-9]*)*$`)

// Candidate is a provisional USB structure match, not firmware identification.
// It supplies no Profile, transport, or permission for device access.
// PhysicalPath is caller-supplied provenance, not a verified device association.
type Candidate struct {
	PhysicalPath  string
	VID           uint16
	PID           uint16
	Revision      uint16
	Configuration byte
	Interfaces    [4]CandidateInterface
}

type CandidateInterface struct {
	Number                 byte
	AlternateSetting       byte
	Class                  byte
	Subclass               byte
	Protocol               byte
	HIDVersion             uint16
	ReportDescriptorLength uint16
	Endpoint               CandidateEndpoint
}

type CandidateEndpoint struct {
	Address       byte
	Attributes    byte
	MaxPacketSize uint16
	Interval      byte
}

// MatchCandidate checks caller-supplied Linux USB descriptor bytes without I/O.
// It accepts only the exact published single-configuration structure. Missing,
// extra, malformed, or changed descriptors fail closed before field extraction.
// Report lengths do not prove report contents, control counts, or KriomantFE.
func MatchCandidate(physicalPath string, descriptors []byte) (Candidate, error) {
	if len(physicalPath) > 255 || !physicalUSBPath.MatchString(physicalPath) {
		return Candidate{}, fmt.Errorf("invalid Linux USB physical path")
	}
	raw := string(descriptors)
	is8890 := len(descriptors) == len(candidateUSBDescriptor) && raw == candidateUSBDescriptor
	is514C := len(descriptors) == len(candidate8851USBDescriptor) && (raw == candidate8851USBDescriptor || raw == candidate8850USBDescriptor)
	if !is8890 && !is514C {
		return Candidate{}, fmt.Errorf("USB descriptors do not match a supported CH57x candidate structure")
	}
	candidate := Candidate{
		PhysicalPath:  physicalPath,
		VID:           binary.LittleEndian.Uint16(descriptors[8:10]),
		PID:           binary.LittleEndian.Uint16(descriptors[10:12]),
		Revision:      binary.LittleEndian.Uint16(descriptors[12:14]),
		Configuration: descriptors[23],
	}
	if is514C {
		candidate.Interfaces[0] = parseCandidateInterface(descriptors[27:59])
		candidate.Interfaces[1] = parseCandidateInterface(descriptors[59:84])
		return candidate, nil
	}
	for i := range candidate.Interfaces {
		candidate.Interfaces[i] = parseCandidateInterface(descriptors[27+i*25 : 27+(i+1)*25])
	}
	return candidate, nil
}

func parseCandidateInterface(d []byte) CandidateInterface {
	return CandidateInterface{
		Number:                 d[2],
		AlternateSetting:       d[3],
		Class:                  d[5],
		Subclass:               d[6],
		Protocol:               d[7],
		HIDVersion:             binary.LittleEndian.Uint16(d[11:13]),
		ReportDescriptorLength: binary.LittleEndian.Uint16(d[16:18]),
		Endpoint: CandidateEndpoint{
			Address:       d[20],
			Attributes:    d[21],
			MaxPacketSize: binary.LittleEndian.Uint16(d[22:24]),
			Interval:      d[24],
		},
	}
}
