// Command ejquick is the EJQuick dictionary search tool. With a
// positional query argument it runs one CLI search; without one it starts
// the interactive TUI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/normalize"
	"github.com/simosako/ejquick/internal/search"
	"github.com/simosako/ejquick/internal/tui"
)

// version is stamped at build time with -ldflags.
var version = "dev"

const usage = `Usage: ejquick [options] [query]

EJQuick dictionary search. With a query argument it prints the results
and exits; without one it starts the interactive TUI.

Options:
  -d, --dictionary <eiji|waei>    dictionary to search
  -c, --config <path>             config file path
      --limit <1..500>            result limit for this process
      --format <plain|jsonl>      output format (CLI search only)
      --debug                     enable debug logging to the log file
  -h, --help                      show this help
  -v, --version                   show version

Exit codes: 0 results found, 1 no results, 2 error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	opts, query, queryPresent, done, err := parseArgs(args, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick: %v\n\n%s", err, usage)
		return 2
	}
	if done {
		return 0
	}
	if !queryPresent && opts.format != "" {
		fmt.Fprintf(stderr, "ejquick: --format requires a query argument\n\n%s", usage)
		return 2
	}

	// Best-effort log file: failures only warn on stderr.
	logger := logging.Open(opts.debug)
	defer logger.Close()

	cfg, err := loadConfig(opts.configPath, opts.configPathSet)
	if err != nil {
		logger.Error("startup: %v", err)
		fmt.Fprintf(stderr, "ejquick: %v\n", err)
		return 2
	}
	if opts.limit > 0 {
		cfg.Search.MaxResults = opts.limit
	}

	if !queryPresent {
		if err := tui.Run(cfg, logger); err != nil {
			logger.Error("tui: %v", err)
			fmt.Fprintf(stderr, "ejquick: %v\n", err)
			return 2
		}
		return 0
	}

	dt := opts.dictionary
	if dt == "" {
		dt = cfg.DefaultDict()
	}
	normalized, err := normalize.Normalize(dt, query)
	if err != nil {
		fmt.Fprintf(stderr, "ejquick: %v\n", err)
		return 2
	}
	if normalized == "" {
		fmt.Fprintf(stderr, "ejquick: query must not be empty after normalization\n\n%s", usage)
		return 2
	}

	return runCLISearch(cfg, opts, query, normalized, logger, stdout, stderr)
}

// cliOptions are parsed command-line options.
type cliOptions struct {
	dictionary    dictionary.Type
	configPath    string
	configPathSet bool
	limit         int
	format        string
	debug         bool
}

// parseArgs returns the options, positional query, and whether that query
// was present. done is true when help or version output has already been
// printed and the caller should exit successfully.
func parseArgs(args []string, stdout io.Writer) (*cliOptions, string, bool, bool, error) {
	opts := &cliOptions{
		dictionary: "",
		format:     "",
	}
	var query string
	queryPresent := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			// Everything after -- is positional.
			rest := args[i+1:]
			if len(rest) > 1 || (queryPresent && len(rest) > 0) {
				return nil, "", false, false, fmt.Errorf("expected exactly one query argument")
			}
			if len(rest) == 1 {
				query = rest[0]
				queryPresent = true
			}
			goto done
		case arg == "-h" || arg == "--help":
			fmt.Fprint(stdout, usage)
			return nil, "", false, true, nil
		case arg == "-v" || arg == "--version":
			fmt.Fprintf(stdout, "ejquick %s\n", version)
			return nil, "", false, true, nil
		case arg == "-d" || arg == "--dictionary":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", false, false, err
			}
			dt, err := dictionary.ParseType(v)
			if err != nil {
				return nil, "", false, false, fmt.Errorf("--dictionary: %w", err)
			}
			opts.dictionary = dt
		case arg == "-c" || arg == "--config":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", false, false, err
			}
			opts.configPath = v
			opts.configPathSet = true
		case arg == "--limit":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", false, false, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > search.HardMaxResults {
				return nil, "", false, false, fmt.Errorf("--limit must be 1..%d", search.HardMaxResults)
			}
			opts.limit = n
		case arg == "--format":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", false, false, err
			}
			if v != "plain" && v != "jsonl" {
				return nil, "", false, false, fmt.Errorf("--format must be plain or jsonl")
			}
			opts.format = v
		case arg == "--debug":
			opts.debug = true
		case strings.HasPrefix(arg, "-"):
			return nil, "", false, false, fmt.Errorf("unknown option %q", arg)
		default:
			if queryPresent {
				return nil, "", false, false, fmt.Errorf("expected exactly one query argument, got multiple")
			}
			query = arg
			queryPresent = true
		}
	}
done:
	return opts, query, queryPresent, false, nil
}

// nextValue consumes the value that follows a value-taking option.
func nextValue(args []string, i *int, name string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("option %s requires a value", name)
	}
	*i++
	return args[*i], nil
}

// loadConfig loads the explicit config file or the default one. An
// explicit path must be readable; the default config may be absent (all
// defaults then apply).
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

// runCLISearch executes one search and prints the results. It returns
// the process exit code.
func runCLISearch(cfg *config.Config, opts *cliOptions, query, normalizedQuery string, logger *logging.Logger, stdout, stderr io.Writer) int {
	dt := opts.dictionary
	if dt == "" {
		dt = cfg.DefaultDict()
	}

	repo, err := search.OpenRepository(cfg.Database(dt), dt)
	if err != nil {
		logger.Error("cli: open %s: %v", dt, err)
		fmt.Fprintf(stderr, "ejquick: %v\n", err)
		return 2
	}
	defer repo.Close()

	svc, err := search.NewService(repo, cfg.Search.MaxResults)
	if err != nil {
		logger.Error("cli: service: %v", err)
		fmt.Fprintf(stderr, "ejquick: %v\n", err)
		return 2
	}

	start := time.Now()
	entries, err := svc.Search(context.Background(), query)
	const requestID = 1
	if err != nil {
		logger.Error("cli search dict=%s request=%d query=%q: %v", dt, requestID, normalizedQuery, err)
		fmt.Fprintf(stderr, "ejquick: %v\n", err)
		return 2
	}
	logger.Debug("cli search dict=%s request=%d query=%q results=%d elapsed=%s",
		dt, requestID, normalizedQuery, len(entries), time.Since(start).Round(time.Microsecond))
	if len(entries) == 0 {
		return 1
	}

	format := opts.format
	if format == "" {
		format = "plain"
	}
	var wErr error
	if format == "jsonl" {
		wErr = writeJSONL(stdout, dt, entries)
	} else {
		wErr = writePlain(stdout, entries)
	}
	if wErr != nil {
		fmt.Fprintf(stderr, "ejquick: write output: %v\n", wErr)
		return 2
	}
	return 0
}

// writePlain prints headword, body, and a blank line per entry. The last
// entry also ends with a newline.
func writePlain(w io.Writer, entries []search.Entry) error {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Headword)
		b.WriteByte('\n')
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// jsonlEntry is the fixed JSON Lines schema.
type jsonlEntry struct {
	ID         int64  `json:"id"`
	Dictionary string `json:"dictionary"`
	Headword   string `json:"headword"`
	Body       string `json:"body"`
}

// writeJSONL prints one JSON object per line with standard escaping.
func writeJSONL(w io.Writer, dt dictionary.Type, entries []search.Entry) error {
	enc := json.NewEncoder(w)
	for _, e := range entries {
		if err := enc.Encode(jsonlEntry{
			ID:         e.ID,
			Dictionary: dt.String(),
			Headword:   e.Headword,
			Body:       e.Body,
		}); err != nil {
			return err
		}
	}
	return nil
}
