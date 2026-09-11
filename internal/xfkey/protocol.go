package xfkey

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const evidenceStatus = "host-derived, hardware-unverified"

type vendorPacket [64]byte
type configuration [128]byte

type deviceIdentity struct {
	Status     string `json:"status"`
	Model      uint16 `json:"model"`
	Version    uint16 `json:"version"`
	Identifier string `json:"identifier"`
	Raw        string `json:"raw"`
}

func validateReply(b []byte, command byte) error {
	if len(b) != 64 {
		return fmt.Errorf("command %02x: need 64 reply bytes, got %d", command, len(b))
	}
	if b[0] != 0xaf || b[1] != command {
		return fmt.Errorf("command %02x: wrong reply header", command)
	}
	return nil
}

func parseIdentity(b []byte) (deviceIdentity, error) {
	if err := validateReply(b, 1); err != nil {
		return deviceIdentity{}, err
	}
	model := binary.BigEndian.Uint16(b[2:4])
	if model != 0x0112 {
		return deviceIdentity{}, fmt.Errorf("unsupported model %04x, require 0112", model)
	}
	return deviceIdentity{evidenceStatus, model, binary.BigEndian.Uint16(b[4:6]), hex.EncodeToString(b[6:10]), hex.EncodeToString(b)}, nil
}

type Readback struct {
	Status        string    `json:"status"`
	Raw           [3]string `json:"raw_replies"`
	Configuration string    `json:"configuration_hex"`
	RGB           RGBInfo   `json:"rgb"`
}

type RGBInfo struct {
	Stored byte   `json:"stored_byte"`
	Index  int    `json:"api_index"`
	Label  string `json:"vendor_label_hardware_unverified"`
	Red    byte   `json:"red"`
	Green  byte   `json:"green"`
	Blue   byte   `json:"blue"`
}

var rgbLabels = [...]string{"Full-colour gradient", "Single-colour steady", "Single-colour flowing", "Flash on click", "Neon flowing", "Lights off", "On while pressed, off on release", "Toggle on click"}

func describeRGB(c configuration) RGBInfo {
	index := int(c[124]) - 1
	label := "unknown"
	if index >= 0 && index < len(rgbLabels) {
		label = rgbLabels[index]
	}
	return RGBInfo{c[124], index, label, c[125], c[126], c[127]}
}

func parseReadback(replies [3][]byte) (Readback, error) {
	var result Readback
	var c configuration
	for i, reply := range replies {
		if err := validateReply(reply, byte(6+i)); err != nil {
			return Readback{}, err
		}
		result.Raw[i] = hex.EncodeToString(reply)
	}
	copy(c[0:62], replies[0][2:64])
	copy(c[62:124], replies[1][2:64])
	copy(c[124:128], replies[2][2:6])
	result.Status = evidenceStatus
	result.Configuration = hex.EncodeToString(c[:])
	result.RGB = describeRGB(c)
	return result, nil
}

func replacement(key string, modifiers, trigger, rgbMode, red, green, blue int, acknowledged bool) (configuration, error) {
	var c configuration
	if !acknowledged {
		return c, fmt.Errorf("--replace-all is required: unknown fields will be reset to zero")
	}
	var usage byte
	switch key {
	case "enter":
		usage = 0x28
	case "f13":
		usage = 0x68
	default:
		return c, fmt.Errorf("supported keys: enter, f13")
	}
	if modifiers < 0 || modifiers > 15 {
		return c, fmt.Errorf("modifiers must be 0..15 (left Ctrl/Shift/Alt/GUI)")
	}
	if trigger < 1 || trigger > 3 {
		return c, fmt.Errorf("trigger must be 1..3 (press/release/both)")
	}
	if rgbMode < 0 || rgbMode > 7 {
		return c, fmt.Errorf("RGB mode must be an explicit index 0..7")
	}
	for _, v := range []int{red, green, blue} {
		if v < 0 || v > 255 {
			return c, fmt.Errorf("each RGB channel must be explicit and 0..255")
		}
	}
	copy(c[:5], []byte{0, byte(trigger), byte(modifiers), 1, usage})
	copy(c[124:], []byte{byte(rgbMode + 1), byte(red), byte(green), byte(blue)})
	return c, nil
}

func preview(c configuration) [5]vendorPacket {
	var packets [5]vendorPacket
	packets[0][0], packets[0][1] = 0xaf, 1
	for i, offset := range []int{0, 60, 120} {
		p := &packets[i+1]
		count := min(60, 128-offset)
		p[0], p[1], p[2], p[3] = 0xaf, 2, byte(offset), byte(count)
		copy(p[4:], c[offset:offset+count])
	}
	packets[4][0], packets[4][1] = 0xaf, 4
	return packets
}

func hidrawOutput(p vendorPacket) [65]byte {
	var output [65]byte
	copy(output[1:], p[:])
	return output
}

func validateEcho(sent vendorPacket, reply []byte) error {
	if len(reply) != 64 || !bytes.Equal(sent[:], reply) {
		return fmt.Errorf("write echo mismatch: stop without commit or retry")
	}
	return nil
}
