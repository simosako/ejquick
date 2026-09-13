// Package gui implements the EJQuick desktop GUI frontend (Qt 6 Widgets
// via MIQT). This file and its test are intentionally free of Qt
// imports, so the plain `go test ./...` and `go vet ./...` runs exercise
// them without a Qt toolchain; the Qt-dependent code in this package
// carries the `gui` build tag (design D35).
package gui

import (
	"fmt"
	"strings"
)

// Action is what the process should do after parsing the command line.
type Action int

const (
	// ActionRun starts the GUI.
	ActionRun Action = iota
	// ActionHelp prints usage and exits successfully.
	ActionHelp
	// ActionVersion prints the version and exits successfully.
	ActionVersion
)

// Options are the parsed ejquick-gui command-line options. Their meaning
// matches the existing ejquick CLI where they overlap (design D25): the
// GUI accepts no positional arguments, no initial query, and no
// per-run search options.
type Options struct {
	// ConfigPath is the value given to --config.
	ConfigPath string
	// ConfigPathSet reports whether --config was given.
	ConfigPathSet bool
	// Debug enables debug logging to the log file.
	Debug bool
}

// Usage is the ejquick-gui help text.
const Usage = `Usage: ejquick-gui [options]

Options:
  -c, --config <path>  config file path
      --debug          enable debug logging to the log file
  -h, --help           show help
  -v, --version        show version
`

// Parse parses the ejquick-gui command line. Help and version win as
// soon as they appear, mirroring the ejquick CLI parser.
func Parse(args []string) (*Options, Action, error) {
	opts := &Options{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			// Anything after -- would be positional, which the GUI
			// does not accept (design D25).
			if i+1 < len(args) {
				return nil, ActionRun, fmt.Errorf("unexpected positional argument %q", args[i+1])
			}
			return opts, ActionRun, nil
		case arg == "-h" || arg == "--help":
			return nil, ActionHelp, nil
		case arg == "-v" || arg == "--version":
			return nil, ActionVersion, nil
		case arg == "-c" || arg == "--config":
			if i+1 >= len(args) {
				return nil, ActionRun, fmt.Errorf("option %s requires a value", arg)
			}
			i++
			opts.ConfigPath = args[i]
			opts.ConfigPathSet = true
		case arg == "--debug":
			opts.Debug = true
		case strings.HasPrefix(arg, "-"):
			return nil, ActionRun, fmt.Errorf("unknown option %q", arg)
		default:
			return nil, ActionRun, fmt.Errorf("unexpected positional argument %q", arg)
		}
	}
	return opts, ActionRun, nil
}
