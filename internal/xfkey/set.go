package xfkey

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type setCommand struct {
	outputOptions
	settingsOptions
	Arguments   []string `arg:"" optional:"" name:"settings" help:"Explicit setting=value assignments, for example key=f13 light=steady:0000ff."`
	Device      string   `help:"Physical USB path. Required when multiple devices match."`
	DryRun      bool     `help:"Read the device and preview changes without saving captures or writing settings."`
	Yes         bool     `help:"Skip confirmation only. All safety checks remain."`
	CaptureRoot string   `help:"Absolute directory for backups." type:"path"`
}

type setAccess struct {
	query       func(string, bool) (CaptureResult, error)
	apply       func(string, string, ApplyTarget, Changes, bool, func(configuration, configuration, string) (bool, error), ...bool) (ApplyResult, error)
	interactive func(io.Reader) bool
}

func (c *setCommand) Run(rt *cliRuntime) error {
	return c.run(rt, setAccess{liveQuery, liveApply, readerInteractive})
}

func (c *setCommand) run(rt *cliRuntime, access setAccess) error {
	changes, err := settingsChanges(c.settingsOptions)
	if err != nil {
		return err
	}
	if !c.DryRun && !c.Yes && !access.interactive(rt.in) {
		return fmt.Errorf("set requires interactive input or --yes; use --dry-run to read and preview without saving; no device access")
	}
	observed, err := access.query(c.Device, true)
	if err != nil {
		return err
	}
	target := targetToken{observed.PhysicalPath, "0112", observed.Identity.Identifier, fmt.Sprintf("%04x", observed.Identity.Version)}
	expected := ApplyTarget{target.Identifier, target.Version}
	if err := expected.validate(); err != nil {
		return err
	}
	if !expected.matches(observed.Identity) {
		return fmt.Errorf("unsupported device identity")
	}
	if observed.Readback == nil {
		return fmt.Errorf("device settings are missing")
	}
	raw, err := hex.DecodeString(observed.Readback.Configuration)
	if err != nil {
		return err
	}
	var current configuration
	if len(raw) != len(current) {
		return fmt.Errorf("device settings need exactly 128 bytes")
	}
	copy(current[:], raw)
	intended, err := changeConfiguration(current, changes)
	if err != nil {
		return err
	}
	result := ApplyResult{Outcome: "no-op", Changes: settingViews(current, intended)}
	out := rt.out
	if c.JSON {
		out = rt.errOut
	}
	h := newHuman(out)
	h.heading("Proposed settings")
	h.field("USB path", target.PhysicalPath)
	h.changes(result.Changes)
	h.line("", "Unspecified settings stay unchanged.")
	finish := func(result ApplyResult, err error) error {
		return (&applyCommand{outputOptions: c.outputOptions}).finishCommand(rt, target, result, err, "set")
	}
	if c.DryRun {
		result.Outcome = "dry-run"
		h.line("", "Dry-run read the device. No files saved or settings written.")
		return finish(result, nil)
	}
	if len(result.Changes) == 0 {
		return finish(result, nil)
	}
	h.line("33", "Warning: this changes stored settings.")
	if !c.Yes {
		writeConfirmationPrompt(rt, c.JSON)
		line, err := bufio.NewReader(rt.in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if !exactWriteConfirmation(line) {
			result.Outcome = "cancelled"
			return finish(result, nil)
		}
	}
	root, err := resolveCaptureRoot(c.CaptureRoot)
	if err != nil {
		return err
	}
	// The backup query must still match the plan approved before capture creation.
	guard := func(fresh, proposed configuration, _ string) (bool, error) {
		if fresh != current || proposed != intended {
			return false, fmt.Errorf("settings changed after confirmation; no settings write sent; run wonkey-dev show before a new set")
		}
		return true, nil
	}
	result, err = access.apply(target.PhysicalPath, root, expected, changes, true, guard, true)
	return finish(result, err)
}
