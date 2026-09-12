package xfkey

import (
	"io"
	"strings"

	"github.com/alecthomas/kong"
)

const settingsReference = `Settings (setting=value):
  key=KEY           Key action: enter or f13. No other keys are supported.
  trigger=WHEN      Send the key action on press, release, or both.
  modifiers=LIST    none, or comma-separated ctrl,shift,alt,gui without repeats.
                    gui is the Super/Windows/Command modifier.
  lighting=MODE     Set only the lighting mode from the list below.
  colour=RGB        Set only the colour: six hexadecimal RGB digits, RRGGBB
                    or #RRGGBB. For example, 0000ff is blue.
  light=MODE:RGB    Set both lighting and colour, for example steady:0000ff.

Supply each setting once. Do not combine light= with lighting= or colour=.
Unspecified settings stay unchanged. Names of values ignore letter case.

Lighting modes:
  gradient          Full-colour gradient
  steady            Single-colour steady
  flowing           Single-colour flowing
  flash             Flash on click
  neon              Neon flowing
  off               Lights off
  held              On while pressed, off on release
  toggle            Toggle on click
These descriptions are vendor labels. Most effects are not verified on hardware.
There are no brightness or speed options.
`

const setSafety = `Saving settings:
  set reads one device, shows changes, then asks for confirmation.
  Type exactly write to confirm. A blank answer cancels.
  Without --yes, confirmation requires a terminal.
  Before writing, set saves and checks a new backup.
  If the device identity or settings change after confirmation, set stops.
  If nothing changes or you cancel, set writes no settings.
`

const setOptions = `  --dry-run             set only. --dry-run reads the device, so it is not offline.
                        Preview changes. Save no files and write no settings.
  --yes                 set only. Skip confirmation, not safety checks or backup.
  --capture-root DIR    set only. Absolute directory for new backups.
`

const backupReference = `Backup location (first non-empty value wins):
  --capture-root, then WONKEY_CAPTURE_ROOT, then XFKEY_CAPTURE_ROOT,
  then $HOME/.local/state/wonkey/captures.
`

const setExamples = `  wonkey-dev set key=f13
  wonkey-dev set light=steady:0000ff
  wonkey-dev set key=f13 light=steady:0000ff
  wonkey-dev set trigger=release modifiers=ctrl,shift
  wonkey-dev set lighting=off
  wonkey-dev set colour=#0000ff
  wonkey-dev set key=f13 --dry-run
  wonkey-dev set --device 1-1.2 key=f13
  wonkey-dev set key=f13 --yes --capture-root /absolute/backups --json
`

const landingPage = `WonKey  One key. Your rules.
Developer tools for an XFKEY One Key Max (0112).

Usage: wonkey-dev <command> [options]

Commands:
  show              Read current settings. No files created.
  set               Save explicit settings, with backup and confirmation.
  advanced          Device discovery, saved plans and protocol tools.

` + settingsReference + `
` + setSafety + `
Options (developer commands only):
  --device PHYSICAL_PATH
                        show, set. Select a physical USB path, such as 1-1.2.
                        Selects the only compatible device when omitted.
                        Required when multiple devices match.
` + setOptions + `  --json                show, set, advanced devices, advanced plan.
                        Write one stable JSON value to stdout.
                        Diagnostics and confirmation prompts stay on stderr.
                        Advanced protocol tools always produce JSON.
  -h, --help            All commands. Show help without device access.

` + backupReference + `
Examples (show and set access a live device, including --dry-run):
  wonkey-dev show
` + setExamples + `
Advanced examples:
  wonkey-dev advanced devices
  wonkey-dev advanced plan ./capture light=steady:0000ff
The plan example reads a saved backup offline. Device discovery reads metadata.

For command help, run "wonkey-dev <command> --help".
`

const setPage = `Usage: wonkey-dev set SETTING=VALUE [SETTING=VALUE ...] [options]
Read, preview and save explicit settings on one compatible device.

` + settingsReference + `
` + setSafety + `
Options:
  --device PHYSICAL_PATH
                        Select a physical USB path, such as 1-1.2.
                        Selects the only compatible device when omitted.
                        Required when multiple devices match.
` + setOptions + `  --json                Write one stable JSON value to stdout.
                        Diagnostics and confirmation prompts stay on stderr.
  -h, --help            Show this help without device access.

` + backupReference + `
Examples (these access a live device, including --dry-run):
` + setExamples

func printHelp(options kong.HelpOptions, ctx *kong.Context) error {
	if selected := ctx.Selected(); selected != nil {
		if selected.Name == "set" {
			_, err := io.WriteString(ctx.Stdout, setPage)
			return err
		}
		return kong.DefaultHelpPrinter(options, ctx)
	}
	return printLanding(ctx.Stdout)
}

func printLanding(out io.Writer) error {
	return printHumanHelp(out, landingPage)
}

func printHumanHelp(out io.Writer, text string) error {
	if h := newHuman(out); h.colour {
		heading, rest, _ := strings.Cut(text, "\n")
		text = h.style("35", heading) + "\n" + rest
	}
	_, err := io.WriteString(out, text)
	return err
}
