//go:build gui

package main

import (
	"fmt"
	"os"

	"github.com/simosako/ejquick/internal/gui"
	"github.com/simosako/ejquick/internal/logging"
)

func main() {
	opts, done, err := parseArgs(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick-gui: %v\n\n%s", err, usage)
		os.Exit(2)
	}
	if done {
		return
	}

	logger := logging.Open(opts.debug)
	exitCode := gui.Run(gui.Options{
		Arguments:    []string{os.Args[0]},
		ConfigPath:   opts.configPath,
		ExplicitPath: opts.configPathSet,
		Logger:       logger,
	})
	if err := logger.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "ejquick-gui: close log: %v\n", err)
	}
	os.Exit(exitCode)
}
