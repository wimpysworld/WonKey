package xfkey

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
)

var errWriteRequired = errors.New("apply requires explicit --write; no device access")

func flags(name string, stderr io.Writer) *flag.FlagSet {
	f := flag.NewFlagSet("wonkey "+name, flag.ContinueOnError)
	f.SetOutput(stderr)
	return f
}

func parseFlags(f *flag.FlagSet, args []string) error {
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", f.Args())
	}
	return nil
}

func decodeReply(value string) ([]byte, error) {
	if len(value) != 128 {
		return nil, fmt.Errorf("reply needs exactly 128 hexadecimal characters (64 bytes)")
	}
	return hex.DecodeString(value)
}

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: wonkey inspect | identify | readback | plan | apply | preview | parse-identify | parse-readback (see README.md)")
	}
	output := json.NewEncoder(stdout)
	output.SetIndent("", "  ")
	switch args[0] {
	case "plan", "apply":
		f := flags(args[0], stderr)
		capture := f.String("capture", "", "saved complete capture directory, plan only")
		path := f.String("path", "", "exact USB topology path, required for apply")
		root := f.String("capture-root", "", "existing absolute directory for a new backup")
		write := f.Bool("write", false, "explicitly permit settings upload and commit")
		identifier := f.String("expect-identifier", "", "expected four identifier bytes in hex")
		version := f.String("expect-version", "", "expected version in big-endian hex")
		key := f.String("key", "", "enter or f13; omitted fields are preserved")
		values := make(map[string]*int)
		for name := range settingsFields {
			if name != "key" {
				values[name] = f.Int(name, -1, "explicit value only; see README")
			}
		}
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		if args[0] == "apply" && !*write {
			return errWriteRequired
		}
		changes := Changes{}
		var keyErr error
		f.Visit(func(field *flag.Flag) {
			if field.Name == "key" {
				switch *key {
				case "enter":
					changes["key"] = 0x28
				case "f13":
					changes["key"] = 0x68
				default:
					keyErr = fmt.Errorf("supported keys: enter, f13")
				}
			} else if value, ok := values[field.Name]; ok {
				changes[field.Name] = *value
			}
		})
		if keyErr != nil {
			return keyErr
		}
		if err := changes.validate(); err != nil {
			return err
		}
		if args[0] == "plan" {
			if *capture == "" || *write || *path != "" || *root != "" || *identifier != "" || *version != "" {
				return fmt.Errorf("plan requires --capture and settings only, never live flags")
			}
			_, current, err := loadCapture(*capture)
			if err != nil {
				return err
			}
			intended, err := changeConfiguration(current, changes)
			if err != nil {
				return err
			}
			return output.Encode(settingsPlan(current, intended))
		}
		if *capture != "" {
			return fmt.Errorf("apply always takes a new live backup; --capture is plan-only")
		}
		result, err := liveApply(*path, *root, ApplyTarget{*identifier, *version}, changes, *write)
		if encodeErr := output.Encode(result); encodeErr != nil {
			return errors.Join(err, encodeErr)
		}
		return err
	case "identify", "readback":
		f := flags(args[0], stderr)
		path := f.String("path", "", "USB physical path, required on ambiguity")
		root := f.String("capture-root", "", "existing absolute directory for a new exclusive capture")
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		if *root == "" {
			return fmt.Errorf("--capture-root is required before any device access")
		}
		result, err := liveCapture(*path, *root, args[0] == "readback")
		if err != nil {
			return err
		}
		return output.Encode(result)
	case "inspect":
		f := flags("inspect", stderr)
		path := f.String("path", "", "USB physical path, for example 1-1.2; never the serial")
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		candidates, err := discover("/sys/bus/usb/devices", "/dev")
		if err != nil {
			return err
		}
		selected, selectionErr := selectCandidate(candidates, *path)
		result := struct {
			Status         string      `json:"status"`
			Candidates     []Candidate `json:"candidates"`
			Selected       string      `json:"selected_physical_path,omitempty"`
			PermissionNote string      `json:"permission_note"`
		}{Status: evidenceStatus, Candidates: candidates, PermissionNote: "Node metadata only, not opened. Mode/owner do not include ACL or prove access. No permission changes made."}
		if selected != nil {
			result.Selected = selected.PhysicalPath
		}
		if err := output.Encode(result); err != nil {
			return err
		}
		return selectionErr
	case "preview":
		f := flags("preview", stderr)
		ack := f.Bool("replace-all", false, "acknowledge that unknown configuration fields reset to zero")
		key := f.String("key", "", "enter or f13")
		modifiers := f.Int("modifiers", -1, "0..15: left Ctrl=1, Shift=2, Alt=4, GUI=8")
		trigger := f.Int("trigger", -1, "1 press, 2 release, 3 both")
		mode := f.Int("rgb-mode", -1, "vendor API index 0..7, hardware-unverified")
		red := f.Int("red", -1, "0..255")
		green := f.Int("green", -1, "0..255")
		blue := f.Int("blue", -1, "0..255")
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		c, err := replacement(*key, *modifiers, *trigger, *mode, *red, *green, *blue, *ack)
		if err != nil {
			return err
		}
		result := struct {
			Status        string   `json:"status"`
			Warning       string   `json:"warning"`
			Configuration string   `json:"configuration_hex"`
			RGB           RGBInfo  `json:"rgb"`
			Payloads      []string `json:"vendor_payloads_64_bytes_hex"`
			Outputs       []string `json:"hidraw_buffers_65_bytes_hex"`
		}{Status: evidenceStatus, Warning: "OFFLINE ONLY. Complete replacement: all unspecified fields reset to zero. No hardware opened or packets sent.", Configuration: hex.EncodeToString(c[:]), RGB: describeRGB(c)}
		for _, packet := range preview(c) {
			result.Payloads = append(result.Payloads, hex.EncodeToString(packet[:]))
			buffer := hidrawOutput(packet)
			result.Outputs = append(result.Outputs, hex.EncodeToString(buffer[:]))
		}
		return output.Encode(result)
	case "parse-identify":
		f := flags("parse-identify", stderr)
		value := f.String("hex", "", "64-byte vendor reply in hex, without report-ID placeholder")
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		b, err := decodeReply(*value)
		if err != nil {
			return err
		}
		identity, err := parseIdentity(b)
		if err != nil {
			return err
		}
		return output.Encode(identity)
	case "parse-readback":
		f := flags("parse-readback", stderr)
		values := [4]*string{f.String("identify", "", "identify reply hex, required to gate model"), f.String("read6", "", "AF06 reply hex"), f.String("read7", "", "AF07 reply hex"), f.String("read8", "", "AF08 reply hex")}
		if err := parseFlags(f, args[1:]); err != nil {
			return err
		}
		var replies [4][]byte
		for i, value := range values {
			b, err := decodeReply(*value)
			if err != nil {
				return err
			}
			replies[i] = b
		}
		identity, err := parseIdentity(replies[0])
		if err != nil {
			return err
		}
		readback, err := parseReadback([3][]byte{replies[1], replies[2], replies[3]})
		if err != nil {
			return err
		}
		return output.Encode(struct {
			Identity deviceIdentity `json:"identity"`
			Readback Readback       `json:"readback"`
		}{identity, readback})
	default:
		return fmt.Errorf("unknown command %q; no raw sender or firmware commands exist", args[0])
	}
}
