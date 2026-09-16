// Command ejquick-build converts an EIJIRO/WAEIJI CP932 TXT file into a
// read-only SQLite dictionary database for EJQuick.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/simosako/ejquick/internal/builder"
	"github.com/simosako/ejquick/internal/buildinfo"
	"github.com/simosako/ejquick/internal/buildprotocol"
	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
)

const usage = `Usage: ejquick-build [options] <input>

Build an EJQuick dictionary database from a CP932 TXT file.

Arguments:
  <input>               source TXT file (required)

Options:
  --type <eiwa|waei>    dictionary type (required)
  --output <path>       destination SQLite database (default: platform data directory)
  --force               replace an existing output database
  --compact             run VACUUM to minimize the database size
  --machine-protocol 1  use the GUI JSON Lines protocol
  -h, --help            show this help
  -v, --version         show version

Progress and diagnostics go to stderr; stdout stays empty.
Exit codes: 0 success, 1 usage error, 2 build failure.
`

func main() {
	os.Exit(runWithIO(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithIO(args, strings.NewReader(""), stdout, stderr)
}

func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, done, err := parseArgs(args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-build: %v\n\n%s", err, usage)
		return 1
	}
	if done {
		return 0
	}

	builder.SetVersion(buildinfo.Version)
	if opts.machineProtocol != 0 {
		return runMachine(stdin, stdout, stderr, opts)
	}
	_, err = builder.Run(opts.Options)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-build: %v\n", err)
		return 2
	}
	output, err := filepath.Abs(opts.Output)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick-build: resolve output path: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "Output: %s\n", output)
	return 0
}

type commandOptions struct {
	builder.Options
	machineProtocol int
}

// parseArgs returns the options, or done=true when help or version
// output has already been printed (the caller should exit 0).
func parseArgs(args []string, stdout, stderr io.Writer) (*commandOptions, bool, error) {
	var (
		typeStr string
		input   string
		output  string
		force   bool
		compact bool
		machine bool
	)
	inputPresent := false
	outputPresent := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			rest := args[i+1:]
			if len(rest) > 1 || (inputPresent && len(rest) > 0) {
				return nil, false, fmt.Errorf("expected exactly one input TXT argument")
			}
			if len(rest) == 1 {
				input = rest[0]
				inputPresent = true
			}
			goto done
		case arg == "-h" || arg == "--help":
			fmt.Fprint(stdout, usage)
			return nil, true, nil
		case arg == "-v" || arg == "--version":
			fmt.Fprintf(stdout, "ejquick-build %s\n", buildinfo.Version)
			return nil, true, nil
		case arg == "--type":
			value, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, false, err
			}
			typeStr = value
		case arg == "--output":
			value, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, false, err
			}
			output = value
			outputPresent = true
		case arg == "--force":
			force = true
		case arg == "--compact":
			compact = true
		case arg == "--machine-protocol":
			value, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, false, err
			}
			if value != "1" {
				return nil, false, fmt.Errorf("--machine-protocol must be 1")
			}
			machine = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option %q", arg)
		default:
			if inputPresent {
				return nil, false, fmt.Errorf("expected exactly one input TXT argument, got multiple")
			}
			input = arg
			inputPresent = true
		}
	}

done:
	dt, err := dictionary.ParseType(typeStr)
	if err != nil {
		return nil, false, fmt.Errorf("--type: %w", err)
	}
	if !inputPresent || input == "" {
		return nil, false, fmt.Errorf("input TXT argument is required")
	}
	if outputPresent && output == "" {
		return nil, false, fmt.Errorf("--output must not be empty")
	}
	if !outputPresent {
		output = config.DefaultDatabasePath(dt)
	}
	machineProtocol := 0
	if machine {
		machineProtocol = machineProtocolVersion
	}

	return &commandOptions{
		Options: builder.Options{
			Type:     dt,
			Input:    input,
			Output:   output,
			Force:    force,
			Compact:  compact,
			Progress: stderr,
		},
		machineProtocol: machineProtocol,
	}, false, nil
}

const machineProtocolVersion = buildprotocol.Version

type protocolCommand = buildprotocol.Command
type protocolEvent = buildprotocol.Event

func runMachine(stdin io.Reader, stdout, stderr io.Writer, opts *commandOptions) int {
	encoder := json.NewEncoder(stdout)
	writeEvent := func(event protocolEvent) error {
		event.Protocol = machineProtocolVersion
		return encoder.Encode(event)
	}
	if err := writeEvent(protocolEvent{
		Event:          "ready",
		ProductVersion: buildinfo.Version,
		Dictionary:     opts.Type.String(),
	}); err != nil {
		fmt.Fprintf(stderr, "ejquick-build: write machine event: %v\n", err)
		return 2
	}

	commands := newCommandReader(stdin)
	command, err := commands.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			_ = writeEvent(protocolEvent{Event: "cancelled"})
			return 130
		}
		_ = writeEvent(protocolEvent{Event: "failed", Code: "protocol_error"})
		fmt.Fprintf(stderr, "ejquick-build: control protocol: %v\n", err)
		return 2
	}
	if command.Command != "start" {
		_ = writeEvent(protocolEvent{Event: "failed", Code: "protocol_error"})
		fmt.Fprintln(stderr, "ejquick-build: control protocol: first command must be start")
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var (
		controlMu  sync.Mutex
		controlErr error
	)
	go func() {
		for {
			command, err := commands.next()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					controlMu.Lock()
					controlErr = err
					controlMu.Unlock()
				}
				cancel()
				return
			}
			if command.Command != "cancel" {
				controlMu.Lock()
				controlErr = fmt.Errorf("unexpected command %q", command.Command)
				controlMu.Unlock()
				cancel()
				return
			}
			cancel()
			return
		}
	}()

	reporter := func(event builder.Event) error {
		wire := protocolEvent{}
		switch event.Kind {
		case builder.EventPhase:
			wire.Event = "phase"
			wire.Phase = string(event.Phase)
			wire.State = string(event.State)
		case builder.EventProgress:
			wire.Event = "progress"
			wire.Phase = string(event.Phase)
			wire.Lines = event.Lines
			wire.Entries = event.Entries
			wire.Skipped = event.Skipped
		case builder.EventSkipped:
			return nil
		case builder.EventTemporaryCreated:
			wire.Event = "temporary_created"
			wire.Path = event.TemporaryPath
		case builder.EventCompleted:
			return nil
		default:
			return fmt.Errorf("unknown builder event %q", event.Kind)
		}
		return writeEvent(wire)
	}

	stats, buildErr := builder.RunContext(ctx, opts.Options, reporter)
	controlMu.Lock()
	protocolErr := controlErr
	controlMu.Unlock()
	if protocolErr != nil {
		_ = writeEvent(protocolEvent{Event: "failed", Code: "protocol_error"})
		fmt.Fprintf(stderr, "ejquick-build: control protocol: %v\n", protocolErr)
		return 2
	}
	if errors.Is(buildErr, context.Canceled) {
		_ = writeEvent(protocolEvent{Event: "cancelled"})
		return 130
	}
	if buildErr != nil {
		_ = writeEvent(protocolEvent{Event: "failed", Code: string(builder.CodeOf(buildErr))})
		fmt.Fprintf(stderr, "ejquick-build: %v\n", buildErr)
		return 2
	}
	if err := writeEvent(protocolEvent{
		Event:       "completed",
		SourceLines: stats.SourceLines,
		Entries:     stats.Entries,
		Skipped:     stats.Skipped,
		DBSize:      stats.DBSize,
		ElapsedMS:   stats.Elapsed.Milliseconds(),
	}); err != nil {
		fmt.Fprintf(stderr, "ejquick-build: write machine event: %v\n", err)
		return 2
	}
	return 0
}

// nextValue consumes the value that follows a value-taking option.
func nextValue(args []string, i *int, name string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("option %s requires a value", name)
	}
	*i++
	return args[*i], nil
}
