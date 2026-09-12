package xfkey

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/x/term"
)

var errWriteRequired = errors.New("apply requires explicit --write; no device access")

const legacyUnset = -2147483648

type outputOptions struct {
	JSON bool `group:"Output" help:"Write one stable JSON value to stdout. Diagnostics stay on stderr."`
}

type settingsOptions struct {
	Key           string `group:"Settings flags (compatibility)" help:"Set the key action: enter or f13."`
	Trigger       string `group:"Settings flags (compatibility)" help:"Set when the key action occurs: press, release, or both."`
	Modifiers     string `group:"Settings flags (compatibility)" help:"Set no modifier or a comma-separated set of ctrl,shift,alt,gui."`
	Lighting      string `group:"Settings flags (compatibility)" help:"Set the lighting effect: gradient, steady, flowing, flash, neon, off, held, or toggle."`
	Colour        string `group:"Settings flags (compatibility)" help:"Set the lighting colour as RRGGBB or #RRGGBB."`
	LegacyRGBMode int    `name:"rgb-mode" default:"-2147483648" hidden:""`
	LegacyRed     int    `name:"red" default:"-2147483648" hidden:""`
	LegacyGreen   int    `name:"green" default:"-2147483648" hidden:""`
	LegacyBlue    int    `name:"blue" default:"-2147483648" hidden:""`
}

type cliModel struct {
	Set           setCommand           `cmd:"" help:"Read, preview and save explicit settings with backup and confirmation."`
	Advanced      advancedCommand      `cmd:"" help:"Device discovery, saved plans and protocol tools."`
	Devices       devicesCommand       `cmd:"" hidden:""`
	Show          showCommand          `cmd:"" help:"Query current hardware settings without creating files."`
	Plan          planCommand          `cmd:"" hidden:""`
	Apply         applyCommand         `cmd:"" hidden:""`
	Protocol      protocolCommand      `cmd:"" hidden:""`
	Inspect       inspectCommand       `cmd:"" hidden:""`
	Identify      identifyCommand      `cmd:"" hidden:""`
	Readback      readbackCommand      `cmd:"" hidden:""`
	Preview       previewCommand       `cmd:"" hidden:""`
	ParseIdentify parseIdentifyCommand `cmd:"" name:"parse-identify" hidden:""`
	ParseReadback parseReadbackCommand `cmd:"" name:"parse-readback" hidden:""`
}

type advancedCommand struct {
	Devices  devicesCommand  `cmd:"" help:"Find compatible devices and physical paths."`
	Plan     planCommand     `cmd:"" help:"Preview changes from a saved backup offline."`
	Protocol protocolCommand `cmd:"" help:"Offline protocol tools. Always JSON."`
}

type devicesCommand struct{ outputOptions }
type inspectCommand struct {
	Path string `help:"Device path, for example 1-1.2."`
}
type identifyCommand struct {
	Path        string `help:"Device path, required when more than one device matches."`
	CaptureRoot string `help:"Existing absolute directory for a new settings capture." type:"path"`
}
type readbackCommand struct {
	Path        string `help:"Device path, required when more than one device matches."`
	CaptureRoot string `help:"Existing absolute directory for a new settings capture." type:"path"`
}
type showCommand struct {
	outputOptions
	Device string `help:"Physical USB path. Required when multiple devices match."`
	Target bool   `hidden:"" help:"Include the legacy copyable target in human output."`
	Path   string `arg:"" optional:"" help:"Physical USB path, for example 1-1.2. Omit only for a single match."`
}
type planCommand struct {
	outputOptions
	settingsOptions
	Capture   string   `help:"Compatibility form of CAPTURE: a completed apply backup directory."`
	Arguments []string `arg:"" optional:"" name:"capture-and-settings" help:"CAPTURE followed by explicit setting=value assignments."`
}
type applyCommand struct {
	outputOptions
	settingsOptions
	Arguments        []string `arg:"" optional:"" name:"target-and-settings" help:"TARGET from show followed by explicit setting=value assignments."`
	Target           string   `group:"Target" help:"Compatibility form of TARGET from show, binding path, model, identifier and version."`
	CaptureRoot      string   `group:"Automation and storage" help:"Absolute root directory for the new backup settings capture." type:"path"`
	Write            bool     `group:"Write permission" help:"Permit settings upload and commit. Without this flag, apply stops before device access."`
	Yes              bool     `group:"Automation and storage" help:"Skip only the interactive confirmation. All other safety checks remain."`
	Path             string   `hidden:""`
	ExpectIdentifier string   `name:"expect-identifier" hidden:""`
	ExpectVersion    string   `name:"expect-version" hidden:""`
}
type protocolCommand struct {
	Preview       previewCommand       `cmd:"" help:"Build replacement packets from explicit values. Always writes JSON."`
	ParseIdentify parseIdentifyCommand `cmd:"" name:"parse-identify" help:"Parse one identify reply. Always writes JSON."`
	ParseReadback parseReadbackCommand `cmd:"" name:"parse-readback" help:"Parse identify and settings replies. Always writes JSON."`
}
type previewCommand struct {
	ReplaceAll bool   `required:"" help:"Confirm that unspecified configuration fields reset to zero."`
	Key        string `required:"" help:"Key action: enter or f13."`
	Modifiers  int    `required:"" help:"Compatibility modifier mask: 0 to 15; Ctrl=1, Shift=2, Alt=4, GUI=8."`
	Trigger    int    `required:"" help:"Compatibility trigger value: 1=press, 2=release, 3=both."`
	RGBMode    int    `name:"rgb-mode" required:"" help:"Compatibility lighting index: 0=gradient, 1=steady, 2=flowing, 3=flash, 4=neon, 5=off, 6=held, 7=toggle."`
	Red        int    `required:"" help:"Compatibility red channel: 0 to 255."`
	Green      int    `required:"" help:"Compatibility green channel: 0 to 255."`
	Blue       int    `required:"" help:"Compatibility blue channel: 0 to 255."`
}
type parseIdentifyCommand struct {
	Hex string `required:"" help:"Identify reply: exactly 64 bytes as 128 hexadecimal characters."`
}
type parseReadbackCommand struct {
	Identify string `required:"" help:"Identify reply: exactly 64 bytes as 128 hexadecimal characters."`
	Read6    string `name:"read6" required:"" help:"AF06 settings reply: exactly 64 bytes as 128 hexadecimal characters."`
	Read7    string `name:"read7" required:"" help:"AF07 settings reply: exactly 64 bytes as 128 hexadecimal characters."`
	Read8    string `name:"read8" required:"" help:"AF08 settings reply: exactly 64 bytes as 128 hexadecimal characters."`
}

func (*devicesCommand) Help() string {
	return "Reads USB and device-node metadata. It does not open a HID device.\n\nExample: wonkey advanced devices"
}

func (*showCommand) Help() string {
	return "Reads current settings from one device and creates no files. Selects the only compatible device.\n\nExample: wonkey show\nMultiple devices: wonkey show --device 1-1.2\nChange settings: wonkey set key=f13"
}

func (*planCommand) Help() string {
	return "Reads only local files in a settings capture. It does not access hardware or write settings.\n\nNeeds a completed apply backup; show creates no capture.\n\nExample: wonkey advanced plan ./capture light=steady:0000ff"
}

func (*applyCommand) Help() string {
	return "Queries one HID device, creates and validates a new backup settings capture, then writes only explicit changes. It requires --write, a target token from show --target or JSON, and confirmation.\n\nExample: wonkey apply TARGET key=f13 light=steady:0000ff --write\n\nSettings: key=, trigger=, modifiers=, lighting=, colour=, light=mode:hex.\nOnly explicit settings change. Do not mix assignments with settings flags.\n\nWrite permission: --write is required. Type exactly write to confirm; blank cancels.\n\nAutomation and storage: --yes skips confirmation only; --json keeps stdout machine-readable.\nBackup root: --capture-root, then WONKEY_CAPTURE_ROOT, then XFKEY_CAPTURE_ROOT, then $HOME/.local/state/xfkey-captures."
}

func (*protocolCommand) Help() string {
	return "Reads command arguments only. These expert commands do not access hardware and always write one JSON value to stdout."
}

type cliRuntime struct {
	in          io.Reader
	out, errOut io.Writer
}

type CLIError struct {
	Err  error
	Code int
}

func (e *CLIError) Error() string { return e.Err.Error() }
func (e *CLIError) Unwrap() error { return e.Err }

func ExitCode(err error) int {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return 1
}

func Run(args []string, stdout, stderr io.Writer) error {
	return RunIO(args, strings.NewReader(""), stdout, stderr)
}

func RunIO(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "--help" || args[0] == "help")) {
		return printLanding(stdout)
	}
	if len(args) == 1 && args[0] == "advanced" {
		args = []string{"advanced", "--help"}
	}
	model := &cliModel{}
	exitCode := -1
	parser, err := kong.New(model,
		kong.Name("wonkey"),
		kong.Description("Configure the key action and lighting on an XFKEY One Key Max."),
		kong.Writers(stdout, stderr),
		kong.Help(printHelp),
		kong.Exit(func(code int) { exitCode = code }),
		kong.Bind(&cliRuntime{stdin, stdout, stderr}),
	)
	if err != nil {
		return err
	}
	if args[0] == "help" {
		args = append(append([]string{}, args[1:]...), "--help")
	}
	ctx, err := parser.Parse(args)
	if exitCode == 0 {
		return nil
	}
	if err != nil {
		return reportCLIError(model, ctx, args, stdout, stderr, err, 2)
	}
	if err := adaptCLI(model, ctx); err != nil {
		return reportCLIError(model, ctx, args, stdout, stderr, err, 2)
	}
	if err := ctx.Run(); err != nil {
		return reportCLIError(model, ctx, args, stdout, stderr, err, 1)
	}
	return nil
}

type reportedError struct{ error }

func reportCLIError(model *cliModel, ctx *kong.Context, args []string, stdout, stderr io.Writer, err error, code int) error {
	var reported reportedError
	if !errors.As(err, &reported) && parsedJSONMode(model, ctx, args) {
		command := commandName(args)
		outcome := "failed"
		if command == "apply" {
			outcome = "failed-before-device-access"
		}
		if encodeErr := encodeJSON(stdout, envelope{SchemaVersion: 1, Command: command, Outcome: outcome, Changes: []changeView{}, Warnings: []string{err.Error()}}); encodeErr != nil {
			err = errors.Join(err, encodeErr)
		}
	}
	message := strings.TrimSuffix(err.Error(), ".")
	newHuman(stderr).line("31", "Error: "+message+".")
	command := commandName(args)
	if command == "plan" {
		fmt.Fprintf(stderr, "Run \"wonkey %s --help\" for an offline example.\n", commandPath(args))
	} else if command == "" || !knownCommand(command) {
		fmt.Fprintln(stderr, `Run "wonkey --help" for available commands.`)
	} else {
		fmt.Fprintf(stderr, "Run \"wonkey %s --help\" for more information.\n", escapeHuman(commandPath(args)))
	}
	return &CLIError{Err: err, Code: code}
}

func parsedJSONMode(model *cliModel, ctx *kong.Context, args []string) bool {
	command := commandName(args)
	if ctx != nil {
		for _, path := range ctx.Path {
			if path.Flag != nil && path.Flag.Name == "json" {
				return ctx.Value(path).Bool()
			}
		}
	}
	if command == "protocol" || hasExplicitJSON(args) {
		return true
	}
	switch command {
	case "devices":
		return model.Devices.JSON
	case "show":
		return model.Show.JSON
	case "plan":
		return model.Plan.JSON
	case "apply":
		return model.Apply.JSON
	case "set":
		return model.Set.JSON
	default:
		return false
	}
}

func hasExplicitJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}

func commandPath(args []string) string {
	if len(args) > 1 && args[0] == "advanced" {
		return "advanced " + commandPath(args[1:])
	}
	if len(args) > 1 && args[0] == "protocol" {
		switch args[1] {
		case "preview", "parse-identify", "parse-readback":
			return "protocol " + args[1]
		}
	}
	return commandName(args)
}

func knownCommand(command string) bool {
	switch command {
	case "devices", "show", "set", "advanced", "plan", "apply", "protocol", "identify", "preview", "parse-identify", "parse-readback":
		return true
	default:
		return false
	}
}

func commandName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case "advanced":
		if len(args) > 1 && args[1] != "--help" {
			return commandName(args[1:])
		}
		return "advanced"
	case "inspect":
		return "devices"
	case "readback":
		return "show"
	default:
		return args[0]
	}
}

func encodeJSON(w io.Writer, value any) error {
	out := json.NewEncoder(w)
	out.SetIndent("", "  ")
	return out.Encode(value)
}

func (c *devicesCommand) Run(rt *cliRuntime) error {
	candidates, err := discover("/sys/bus/usb/devices", "/dev")
	if err != nil {
		return err
	}
	if c.JSON {
		return encodeJSON(rt.out, struct {
			SchemaVersion int         `json:"schema_version"`
			Command       string      `json:"command"`
			Outcome       string      `json:"outcome"`
			Candidates    []Candidate `json:"candidates"`
		}{1, "devices", "success", candidates})
	}
	newHuman(rt.out).devices(candidates)
	return nil
}

func (c *inspectCommand) Run(rt *cliRuntime) error {
	candidates, err := discover("/sys/bus/usb/devices", "/dev")
	if err != nil {
		return err
	}
	selected, selectionErr := selectCandidate(candidates, c.Path)
	result := struct {
		Status         string      `json:"status"`
		Candidates     []Candidate `json:"candidates"`
		Selected       string      `json:"selected_physical_path,omitempty"`
		PermissionNote string      `json:"permission_note"`
	}{Status: evidenceStatus, Candidates: candidates, PermissionNote: "Node metadata only, not opened. Mode/owner do not include ACL or prove access. No permission changes made."}
	if selected != nil {
		result.Selected = selected.PhysicalPath
	}
	if err := encodeJSON(rt.out, result); err != nil {
		return err
	}
	return selectionErr
}

func runLegacyCapture(rt *cliRuntime, path, root string, readback bool) error {
	if root == "" {
		return fmt.Errorf("--capture-root is required before any device access")
	}
	result, err := liveCapture(path, root, readback)
	if encodeErr := encodeJSON(rt.out, result); encodeErr != nil {
		return errors.Join(err, encodeErr)
	}
	return err
}

func (c *identifyCommand) Run(rt *cliRuntime) error {
	return runLegacyCapture(rt, c.Path, c.CaptureRoot, false)
}

func (c *readbackCommand) Run(rt *cliRuntime) error {
	return runLegacyCapture(rt, c.Path, c.CaptureRoot, true)
}

func (c *showCommand) Run(rt *cliRuntime) error {
	result, err := liveQuery(c.Device, true)
	if err != nil {
		return err
	}
	target := targetToken{result.PhysicalPath, "0112", result.Identity.Identifier, fmt.Sprintf("%04x", result.Identity.Version)}
	token, err := encodeTarget(target)
	if err != nil {
		return err
	}
	var current configuration
	raw, err := hex.DecodeString(result.Readback.Configuration)
	if err != nil {
		return err
	}
	copy(current[:], raw)
	if err := supportedConfiguration(current); err != nil {
		return err
	}
	if c.JSON {
		return encodeJSON(rt.out, envelope{SchemaVersion: 1, Command: "show", Outcome: "success", Device: &target, Target: token, Changes: []changeView{}, Warnings: []string{"Lighting names are vendor labels. Most effects are not verified on hardware."}})
	}
	newHuman(rt.out).show(target, current)
	if c.Target {
		newHuman(rt.out).field("Target", compactTarget(target))
	}
	return nil
}

func modifierName(v byte) string {
	if v == 0 {
		return "none"
	}
	names := []string{}
	for _, x := range []struct {
		name string
		bit  byte
	}{{"ctrl", 1}, {"shift", 2}, {"alt", 4}, {"gui", 8}} {
		if v&x.bit != 0 {
			names = append(names, x.name)
		}
	}
	return strings.Join(names, ",")
}

func (c *planCommand) Run(rt *cliRuntime) error {
	changes, err := settingsChanges(c.settingsOptions)
	if err != nil {
		return err
	}
	capture, current, err := loadCapture(c.Capture)
	if err != nil {
		return err
	}
	intended, err := changeConfiguration(current, changes)
	if err != nil {
		return err
	}
	views := settingViews(current, intended)
	target := targetToken{"", "0112", capture.Identity.Identifier, fmt.Sprintf("%04x", capture.Identity.Version)}
	if c.JSON {
		return encodeJSON(rt.out, envelope{SchemaVersion: 1, Command: "plan", Outcome: map[bool]string{true: "no-op", false: "planned"}[len(views) == 0], Device: &target, Capture: c.Capture, Changes: views, Warnings: []string{"This offline plan cannot authorise a write."}})
	}
	h := newHuman(rt.out)
	h.heading("Offline plan")
	h.field("Capture", c.Capture)
	h.field("Device", "One Key Max 0112")
	h.field("Identifier", target.Identifier)
	h.field("Version", target.Version)
	if len(views) == 0 {
		h.line("", "No changes needed in this backup.")
	} else {
		h.changes(views)
	}
	h.line("", "Unspecified settings stay unchanged. No device access.")
	h.line("33", "This offline plan cannot authorise a write.")
	return nil
}

func settingsChanges(options settingsOptions) (Changes, error) {
	trigger, numericTrigger := strconv.Atoi(options.Trigger)
	modifiers, numericModifiers := strconv.Atoi(options.Modifiers)
	legacy := numericTrigger == nil || numericModifiers == nil || options.LegacyRGBMode != legacyUnset || options.LegacyRed != legacyUnset || options.LegacyGreen != legacyUnset || options.LegacyBlue != legacyUnset
	friendly := (options.Trigger != "" && numericTrigger != nil) || (options.Modifiers != "" && numericModifiers != nil) || options.Lighting != "" || options.Colour != ""
	if legacy && friendly {
		return nil, fmt.Errorf("friendly settings flags cannot be combined with numeric compatibility values")
	}
	var changes Changes
	var err error
	if legacy {
		changes = Changes{}
		if options.Key != "" {
			changes, err = parseSettings(options.Key, "", "", "", "")
		}
	} else {
		changes, err = parseSettings(options.Key, options.Trigger, options.Modifiers, options.Lighting, options.Colour)
	}
	if err != nil {
		return nil, err
	}
	if numericTrigger == nil {
		changes["trigger"] = trigger
	}
	if numericModifiers == nil {
		changes["modifiers"] = modifiers
	}
	for name, value := range map[string]int{"rgb-mode": options.LegacyRGBMode, "red": options.LegacyRed, "green": options.LegacyGreen, "blue": options.LegacyBlue} {
		if value != legacyUnset {
			changes[name] = value
		}
	}
	if err := changes.validate(); err != nil {
		return nil, err
	}
	return changes, nil
}

func (c *applyCommand) Run(rt *cliRuntime) error {
	if !c.Write {
		return errWriteRequired
	}
	var target targetToken
	var err error
	legacy := c.Path != "" || c.ExpectIdentifier != "" || c.ExpectVersion != ""
	if c.Target != "" && legacy {
		return fmt.Errorf("target cannot be combined with compatibility target flags")
	}
	if legacy {
		target = targetToken{PhysicalPath: c.Path, Model: "0112", Identifier: c.ExpectIdentifier, Version: c.ExpectVersion}
		if target.PhysicalPath == "" {
			return fmt.Errorf("compatibility apply requires --path, --expect-identifier, and --expect-version")
		}
		if err := (ApplyTarget{target.Identifier, target.Version}).validate(); err != nil {
			return err
		}
	} else {
		target, err = decodeTarget(c.Target)
		if err != nil {
			return err
		}
	}
	changes, err := settingsChanges(c.settingsOptions)
	if err != nil {
		return err
	}
	root, err := resolveCaptureRoot(c.CaptureRoot)
	if err != nil {
		return err
	}
	if !c.Yes && !readerInteractive(rt.in) {
		return fmt.Errorf("apply requires interactive input or --yes; no device access")
	}
	confirm := func(current, intended configuration, dir string) (bool, error) {
		views := settingViews(current, intended)
		out := rt.out
		if c.JSON {
			out = rt.errOut
		}
		h := newHuman(out)
		h.heading("Ready to write")
		h.field("Target", compactTarget(target))
		h.field("Backup", dir)
		h.changes(views)
		h.line("", "Unspecified settings stay unchanged.")
		h.line("33", "Warning: this changes stored settings.")
		if c.Yes {
			return true, nil
		}
		writeConfirmationPrompt(rt, c.JSON)
		line, e := bufio.NewReader(rt.in).ReadString('\n')
		if e != nil && !errors.Is(e, io.EOF) {
			return false, e
		}
		return exactWriteConfirmation(line), nil
	}
	result, err := liveApply(target.PhysicalPath, root, ApplyTarget{target.Identifier, target.Version}, changes, true, confirm)
	return c.finish(rt, target, result, err)
}

func (c *applyCommand) finish(rt *cliRuntime, target targetToken, result ApplyResult, err error) error {
	return c.finishCommand(rt, target, result, err, "apply")
}

func (c *applyCommand) finishCommand(rt *cliRuntime, target targetToken, result ApplyResult, err error, command string) error {
	if err != nil {
		if c.JSON {
			encodeErr := encodeJSON(rt.out, envelope{SchemaVersion: 1, Command: command, Outcome: result.Outcome, Device: &target, Capture: result.Directory, Changes: result.Changes, WriteAttempted: result.WriteAttempted, ReadbackVerified: result.ReadbackVerified, Warnings: []string{err.Error()}})
			return reportedError{errors.Join(err, encodeErr)}
		}
		newHuman(rt.errOut).apply(result, err)
		return err
	}
	warnings := []string{"Persistence after reconnect is not verified."}
	if result.Outcome == "dry-run" {
		warnings = []string{"Dry-run read the device. No files saved or settings written."}
	}
	if result.CleanupWarning != "" {
		warnings = append(warnings, result.CleanupWarning)
		newHuman(rt.errOut).line("33", "Warning: "+result.CleanupWarning+". Do not retry a successful apply.")
	}
	out := envelope{SchemaVersion: 1, Command: command, Outcome: result.Outcome, Device: &target, Capture: result.Directory, Changes: result.Changes, WriteAttempted: result.WriteAttempted, ReadbackVerified: result.ReadbackVerified, PersistenceVerified: false, Warnings: warnings}
	if c.JSON {
		return encodeJSON(rt.out, out)
	}
	newHuman(rt.out).apply(result, nil)
	return nil
}

func readerInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func writeConfirmationPrompt(rt *cliRuntime, jsonOutput bool) {
	out := rt.out
	if jsonOutput {
		out = rt.errOut
	}
	fmt.Fprint(out, "Type \"write\" to change stored settings [cancel]: ")
}

func exactWriteConfirmation(line string) bool {
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line == "write"
}

func (c *previewCommand) Run(rt *cliRuntime) error {
	config, err := replacement(c.Key, c.Modifiers, c.Trigger, c.RGBMode, c.Red, c.Green, c.Blue, c.ReplaceAll)
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
	}{Status: evidenceStatus, Warning: "OFFLINE ONLY. Complete replacement: all unspecified fields reset to zero. No hardware opened or packets sent.", Configuration: hex.EncodeToString(config[:]), RGB: describeRGB(config), Payloads: []string{}, Outputs: []string{}}
	for _, packet := range preview(config) {
		result.Payloads = append(result.Payloads, hex.EncodeToString(packet[:]))
		buffer := hidrawOutput(packet)
		result.Outputs = append(result.Outputs, hex.EncodeToString(buffer[:]))
	}
	return encodeJSON(rt.out, result)
}
func decodeCLIReply(value string) ([]byte, error) {
	if len(value) != 128 {
		return nil, fmt.Errorf("reply needs exactly 128 hexadecimal characters (64 bytes)")
	}
	return hex.DecodeString(value)
}
func (c *parseIdentifyCommand) Run(rt *cliRuntime) error {
	b, e := decodeCLIReply(c.Hex)
	if e != nil {
		return e
	}
	v, e := parseIdentity(b)
	if e != nil {
		return e
	}
	return encodeJSON(rt.out, v)
}
func (c *parseReadbackCommand) Run(rt *cliRuntime) error {
	values := []string{c.Identify, c.Read6, c.Read7, c.Read8}
	var replies [4][]byte
	for i, v := range values {
		b, e := decodeCLIReply(v)
		if e != nil {
			return e
		}
		replies[i] = b
	}
	id, e := parseIdentity(replies[0])
	if e != nil {
		return e
	}
	r, e := parseReadback([3][]byte{replies[1], replies[2], replies[3]})
	if e != nil {
		return e
	}
	return encodeJSON(rt.out, struct {
		Identity deviceIdentity `json:"identity"`
		Readback Readback       `json:"readback"`
	}{id, r})
}
