package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/simosako/ejquick/internal/buildinfo"
)

const usage = `Usage: ejquick-gui [options]

EJQuick desktop dictionary search.

Options:
  -c, --config <path>  config file path
      --debug          enable debug logging to the log file
  -h, --help           show this help
  -v, --version        show version
`

type options struct {
	configPath    string
	configPathSet bool
	debug         bool
}

func parseArgs(args []string, stdout io.Writer) (options, bool, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-h" || arg == "--help":
			fmt.Fprint(stdout, usage)
			return options{}, true, nil
		case arg == "-v" || arg == "--version":
			fmt.Fprintf(stdout, "ejquick-gui %s\n", buildinfo.Version)
			return options{}, true, nil
		case arg == "-c" || arg == "--config":
			if i+1 >= len(args) {
				return options{}, false, fmt.Errorf("option %s requires a value", arg)
			}
			i++
			opts.configPath = args[i]
			opts.configPathSet = true
		case arg == "--debug":
			opts.debug = true
		case strings.HasPrefix(arg, "-"):
			return options{}, false, fmt.Errorf("unknown option %q", arg)
		default:
			return options{}, false, fmt.Errorf("unexpected positional argument %q", arg)
		}
	}
	return opts, false, nil
}
