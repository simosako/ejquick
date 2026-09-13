//go:build gui

// Command ejquick-gui is the EJQuick desktop GUI frontend built on
// Qt 6 Widgets through MIQT. It shares the dictionary data, the TOML
// config file, and the append-only log file with the existing TUI/CLI.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/gui"
	"github.com/simosako/ejquick/internal/logging"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, action, err := gui.Parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-gui: %v\n\n%s", err, gui.Usage)
		return 2
	}
	switch action {
	case gui.ActionHelp:
		fmt.Fprint(stdout, gui.Usage)
		return 0
	case gui.ActionVersion:
		fmt.Fprintf(stdout, "ejquick-gui %s\n", buildinfo.Version)
		return 0
	}

	// Best-effort log file shared with the TUI/CLI (design D24): a
	// broken log file never prevents startup.
	logger := logging.Open(opts.Debug)
	defer logger.Close()

	cfg, err := loadConfig(opts.ConfigPath, opts.ConfigPathSet)
	if err != nil {
		// M1 interim behavior: report the failure on stderr and exit 2.
		// The startup dialog of design D46 replaces this in the next
		// milestone.
		logger.Error("startup: %v", err)
		fmt.Fprintf(stderr, "ejquick-gui: %v\n", err)
		return 2
	}

	return gui.Run(cfg, logger)
}

// loadConfig has the same semantics as the ejquick CLI: an explicit
// --config file must exist and be readable, while a missing default
// config file means all defaults apply.
func loadConfig(path string, explicit bool) (*config.Config, error) {
	if explicit {
		return config.Load(path)
	}
	defaultPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(defaultPath); err == nil {
		return config.Load(defaultPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat default config %s: %w", defaultPath, err)
	}
	return config.Defaults(), nil
}
