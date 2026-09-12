// Command ejquick-build converts an EIJIRO/WAEIJI CP932 TXT file into a
// read-only SQLite dictionary database for EJQuick.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/dictionary"
)

// version is stamped at build time with -ldflags.
var version = "dev"

const usage = `Usage: ejquick-build [options]

Build an EJQuick dictionary database from a CP932 TXT file.

Options:
  --type <eiji|waei>    dictionary type (required)
  --input <path>        source TXT file (required)
  --output <path>       destination SQLite database (required)
  --force               replace an existing output database
  --compact             run VACUUM to minimize the database size
  -h, --help            show this help
  -v, --version         show version

Progress and diagnostics go to stderr; stdout stays empty.
Exit codes: 0 success, 1 usage error, 2 build failure.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, done, err := parseArgs(args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-build: %v\n\n%s", err, usage)
		return 1
	}
	if done {
		return 0
	}

	builder.SetVersion(version)
	_, err = builder.Run(*opts)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-build: %v\n", err)
		return 2
	}
	return 0
}

// parseArgs returns the options, or done=true when help or version
// output has already been printed (the caller should exit 0).
func parseArgs(args []string, stdout, stderr io.Writer) (*builder.Options, bool, error) {
	fs := flag.NewFlagSet("ejquick-build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	var (
		typeStr  = fs.String("type", "", "dictionary type: eiji or waei")
		input    = fs.String("input", "", "source TXT file")
		output   = fs.String("output", "", "destination database")
		force    = fs.Bool("force", false, "replace existing output")
		compact  = fs.Bool("compact", false, "run VACUUM")
		help     = fs.Bool("h", false, "show help")
		helpLong = fs.Bool("help", false, "show help")
		ver      = fs.Bool("v", false, "show version")
		verLong  = fs.Bool("version", false, "show version")
	)
	if err := fs.Parse(args); err != nil {
		return nil, false, err
	}
	if *help || *helpLong {
		fmt.Fprint(stdout, usage)
		return nil, true, nil
	}
	if *ver || *verLong {
		fmt.Fprintf(stdout, "ejquick-build %s\n", version)
		return nil, true, nil
	}

	dt, err := dictionary.ParseType(*typeStr)
	if err != nil {
		return nil, false, fmt.Errorf("--type: %w", err)
	}
	if *input == "" {
		return nil, false, fmt.Errorf("--input is required")
	}
	if *output == "" {
		return nil, false, fmt.Errorf("--output is required")
	}
	if fs.NArg() > 0 {
		return nil, false, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	return &builder.Options{
		Type:     dt,
		Input:    *input,
		Output:   *output,
		Force:    *force,
		Compact:  *compact,
		Progress: stderr,
	}, false, nil
}
