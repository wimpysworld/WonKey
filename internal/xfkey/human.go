package xfkey

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/term"
)

type human struct {
	out                io.Writer
	colour, trueColour bool
	width              int
}

func newHuman(out io.Writer) human {
	f, ok := out.(*os.File)
	tty := ok && term.IsTerminal(f.Fd())
	h := humanFor(out, tty, os.Getenv)
	if tty {
		if width, _, err := term.GetSize(f.Fd()); err == nil && width > 0 {
			h.width = width
		}
	}
	return h
}

func humanFor(out io.Writer, tty bool, getenv func(string) string) human {
	colour := tty && getenv("NO_COLOR") == "" && getenv("TERM") != "dumb" && getenv("TERM") != ""
	capability := strings.ToLower(getenv("COLORTERM"))
	return human{out, colour, colour && (capability == "truecolor" || capability == "24bit"), 80}
}

func escapeHuman(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			quoted := strconv.QuoteRune(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func (h human) style(code, text string) string {
	if h.colour && code != "" {
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	return text
}
func (h human) line(code, text string) {
	text = escapeHuman(text)
	for h.width > 0 && len([]rune(text)) > h.width {
		runes := []rune(text)
		cut := -1
		for i := 0; i <= h.width; i++ {
			if runes[i] == ' ' {
				cut = i
			}
		}
		if cut <= 0 {
			break
		}
		fmt.Fprintln(h.out, h.style(code, string(runes[:cut])))
		text = string(runes[cut+1:])
	}
	fmt.Fprintln(h.out, h.style(code, text))
}
func (h human) heading(text string) {
	h.line("35", "WonKey")
	h.line("36", text)
}
func (h human) field(label, value string) {
	if h.width > 0 && h.width < 60 {
		fmt.Fprintf(h.out, "  %s\n    %s\n", escapeHuman(label), escapeHuman(value))
		return
	}
	fmt.Fprintf(h.out, "  %-10s %s\n", escapeHuman(label), escapeHuman(value))
}
func (h human) rgb(value string) string {
	text := escapeHuman(value)
	if h.trueColour && len(value) == 7 && value[0] == '#' {
		rgb, err := strconv.ParseUint(value[1:], 16, 24)
		if err == nil {
			return fmt.Sprintf("\x1b[48;2;%d;%d;%dm  \x1b[0m %s", rgb>>16, (rgb>>8)&255, rgb&255, text)
		}
	}
	return text
}
func (h human) changes(views []changeView) {
	if len(views) == 0 {
		h.line("", "No changes needed.")
		return
	}
	h.line("36", "Changes:")
	for _, view := range views {
		before, after := escapeHuman(view.Before), escapeHuman(view.After)
		if view.Setting == "colour" {
			before, after = h.rgb(view.Before), h.rgb(view.After)
		}
		fmt.Fprintf(h.out, "  %s: %s -> %s\n", escapeHuman(view.Setting), before, after)
	}
}
func (h human) devices(candidates []Candidate) {
	h.heading("Device discovery")
	count := 0
	for _, candidate := range candidates {
		if candidate.Compatible {
			count++
		}
	}
	if count == 0 {
		h.line("", "No matching devices found.")
	} else {
		h.line("", fmt.Sprintf("Found %d descriptor-matched device(s).", count))
	}
	for _, candidate := range candidates {
		h.field("USB path", candidate.PhysicalPath)
		h.field("Device", "One Key Max 0112")
		if candidate.Compatible {
			h.field("Match", "exact descriptors")
		} else {
			h.line("33", "Rejected: "+strings.Join(candidate.Reasons, "; "))
		}
		h.field("Node", candidate.VendorNode.Path)
	}
	h.line("33", "Node access was not tested. No HID device opened.")
}
func (h human) show(target targetToken, current configuration) {
	h.heading("Current settings")
	h.field("Device", "One Key Max 0112")
	h.field("USB path", target.PhysicalPath)
	h.field("Key", map[byte]string{0x28: "enter", 0x68: "f13"}[current[4]])
	h.field("Trigger", map[byte]string{1: "press", 2: "release", 3: "both"}[current[1]])
	h.field("Modifiers", modifierName(current[2]))
	h.field("Lighting", []string{"", "gradient", "steady", "flowing", "flash", "neon", "off", "held", "toggle"}[current[124]])
	fmt.Fprintf(h.out, "  %-10s %s\n", "Colour", h.rgb(fmt.Sprintf("#%02X%02X%02X", current[125], current[126], current[127])))
	h.line("", "No files created. The swatch shows configured RGB, not observed lighting.")
	h.line("33", "Lighting names are vendor labels. Most effects are not verified on hardware.")
}
func (h human) apply(result ApplyResult, err error) {
	h.heading("Apply result")
	if err != nil {
		switch {
		case result.Outcome == "readback-mismatch":
			h.line("31", "Readback differs from intended settings. Device state uncertain.")
		case result.ReadbackVerified:
			h.line("31", "Apply failed after readback matched. Record handling failed; do not assume device state.")
		case result.WriteAttempted:
			h.line("31", "Apply failed. Device state uncertain.")
		default:
			h.line("31", "Apply stopped before settings upload.")
		}
		if result.WriteAttempted {
			h.line("33", "A settings write was submitted. A timed-out write can still complete.")
		} else {
			h.line("", "No settings upload or commit sent.")
		}
		if result.Outcome == "failed-before-upload" {
			h.line("", "Stage: backup or preparation. Device query was attempted.")
		} else if result.Outcome == "" {
			h.line("", "Stage: live setup. Device access is not established by this result.")
		} else {
			h.field("Stage", result.Outcome)
		}
		h.line("33", "Stop. Do not retry automatically. No rollback was attempted.")
	} else {
		switch result.Outcome {
		case "no-op", "cancelled":
			message := "No changes needed."
			if result.Outcome == "cancelled" {
				message = "Cancelled."
			}
			if result.Directory != "" {
				message += " Backup saved."
			}
			h.line("", message+" No settings write sent.")
		case "readback-verified":
			h.line("32", "Settings written; readback verified. All 128 bytes match intended settings.")
		default:
			h.field("Result", result.Outcome)
		}
	}
	if result.Directory != "" {
		label := "Backup"
		if err != nil {
			label = "Records"
		}
		h.field(label, result.Directory)
	}
	h.line("33", "Persistence after reconnect is not verified.")
	if err == nil && result.ReadbackVerified {
		h.line("", "Key output and lighting effects need a separate hardware check.")
	}
}
