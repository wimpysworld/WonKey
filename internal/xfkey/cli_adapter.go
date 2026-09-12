package xfkey

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

var settingsFlagNames = []string{"key", "trigger", "modifiers", "lighting", "colour", "rgb-mode", "red", "green", "blue"}

func adaptCLI(model *developerModel, ctx *kong.Context) error {
	command := strings.TrimPrefix(ctx.Command(), "advanced ")
	flags := map[string]bool{}
	for _, path := range ctx.Path {
		if path.Flag != nil {
			name := path.Flag.Name
			if flags[name] && (strings.HasPrefix(command, "plan") || strings.HasPrefix(command, "apply") || strings.HasPrefix(command, "set")) {
				for _, setting := range settingsFlagNames {
					if setting == name {
						return fmt.Errorf("duplicate settings flag --%s; supply each setting once", name)
					}
				}
			}
			flags[name] = true
		}
	}
	if strings.HasPrefix(ctx.Command(), "advanced plan") {
		return adaptSettings(model.Advanced.Plan.Arguments, &model.Advanced.Plan.Capture, "capture", &model.Advanced.Plan.settingsOptions, flags)
	}
	switch {
	case strings.HasPrefix(command, "set"):
		object := ""
		return adaptSettings(model.Set.Arguments, &object, "", &model.Set.settingsOptions, flags)
	case strings.HasPrefix(command, "show"):
		if model.Show.Path != "" {
			if flags["device"] {
				return fmt.Errorf("PATH %q conflicts with --device; use one object", model.Show.Path)
			}
			model.Show.Device = model.Show.Path
		}
	case strings.HasPrefix(ctx.Command(), "plan"):
		return adaptSettings(model.Plan.Arguments, &model.Plan.Capture, "capture", &model.Plan.settingsOptions, flags)
	case strings.HasPrefix(ctx.Command(), "apply"):
		return adaptSettings(model.Apply.Arguments, &model.Apply.Target, "target", &model.Apply.settingsOptions, flags)
	}
	return nil
}

func adaptSettings(args []string, object *string, name string, options *settingsOptions, flags map[string]bool) error {
	flaggedObject := flags[name] || (name == "target" && (flags["path"] || flags["expect-identifier"] || flags["expect-version"]))
	if name != "" && len(args) > 0 && (!strings.Contains(args[0], "=") || !flaggedObject) {
		if flaggedObject {
			return fmt.Errorf("positional %s %q conflicts with flagged object; use one object", name, args[0])
		}
		*object, args = args[0], args[1:]
	}
	if name == "capture" && *object == "" {
		return fmt.Errorf("capture is required; supply a completed apply backup directory")
	}
	if len(args) == 0 {
		return nil
	}
	for _, flag := range settingsFlagNames {
		if flags[flag] {
			return fmt.Errorf("assignment %q cannot be combined with settings flag --%s; use one settings syntax", args[0], flag)
		}
	}
	seen := map[string]bool{}
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok || value == "" {
			return fmt.Errorf("invalid setting %q; use setting=value", arg)
		}
		values := map[string]*string{"key": &options.Key, "trigger": &options.Trigger, "modifiers": &options.Modifiers, "lighting": &options.Lighting, "colour": &options.Colour}
		names := []string{key}
		if key == "light" {
			names = []string{"lighting", "colour"}
		}
		for _, name := range names {
			if values[name] == nil {
				return fmt.Errorf("unknown setting %q; use key, trigger, modifiers, lighting, colour or light", arg)
			}
			if seen[name] {
				return fmt.Errorf("duplicate or overlapping setting %q; supply each setting once", arg)
			}
			seen[name] = true
		}
		if key == "light" {
			mode, colour, ok := strings.Cut(value, ":")
			if !ok || mode == "" || colour == "" {
				return fmt.Errorf("invalid setting %q; use light=mode:RRGGBB", arg)
			}
			options.Lighting, options.Colour = mode, colour
		} else {
			*values[key] = value
		}
	}
	// Assignments use named values, never the numeric compatibility grammar.
	_, err := parseSettings(options.Key, options.Trigger, options.Modifiers, options.Lighting, options.Colour)
	if err != nil {
		return fmt.Errorf("invalid assignments %q: %s", args, strings.ReplaceAll(err.Error(), "--", ""))
	}
	return nil
}
