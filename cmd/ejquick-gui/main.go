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

	// gui.Run loads the configuration itself so config failures can be
	// recovered through the in-process startup dialog (design D39/D46).
	return gui.Run(*opts, logger)
}
