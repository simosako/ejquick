// Command ejquick is the EJQuick dictionary search tool. With a
// positional query argument it runs one CLI search; without one it starts
// the interactive TUI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
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
  -h, --help                      show this help
  -v, --version                   show version

Exit codes: 0 results found, 1 no results, 2 error.
`

func main() {
	opts, query, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick: %v\n\n%s", err, usage)
		os.Exit(2)
	}
	if opts == nil {
		fmt.Fprint(os.Stdout, usage)
		return
	}

	cfg, err := loadConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick: %v\n", err)
		os.Exit(2)
	}
	if opts.limit > 0 {
		cfg.Search.MaxResults = opts.limit
	}

	if query == "" {
		// TUI mode is delivered in a later phase.
		fmt.Fprintln(os.Stderr, "ejquick: TUI is not yet implemented; pass a query to use the CLI search")
		os.Exit(2)
	}

	os.Exit(runCLISearch(cfg, opts, query))
}

// cliOptions are parsed command-line options.
type cliOptions struct {
	dictionary dictionary.Type
	configPath string
	limit      int
	format     string
}

// parseArgs returns the options and the positional query ("" for TUI
// mode). A nil *cliOptions with nil error means help/version was printed.
func parseArgs(args []string) (*cliOptions, string, error) {
	opts := &cliOptions{
		dictionary: "",
		format:     "",
	}
	var query string
	positional := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			// Everything after -- is positional.
			rest := args[i+1:]
			if len(rest) > 0 {
				positional += len(rest) - 1
				query = rest[0]
			}
			if positional > 0 {
				return nil, "", fmt.Errorf("expected exactly one query argument")
			}
			goto done
		case arg == "-h" || arg == "--help":
			return nil, "", nil
		case arg == "-v" || arg == "--version":
			fmt.Fprintf(os.Stdout, "ejquick %s\n", version)
			return nil, "", nil
		case arg == "-d" || arg == "--dictionary":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", err
			}
			dt, err := dictionary.ParseType(v)
			if err != nil {
				return nil, "", fmt.Errorf("--dictionary: %w", err)
			}
			opts.dictionary = dt
		case arg == "-c" || arg == "--config":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", err
			}
			opts.configPath = v
		case arg == "--limit":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > search.HardMaxResults {
				return nil, "", fmt.Errorf("--limit must be 1..%d", search.HardMaxResults)
			}
			opts.limit = n
		case arg == "--format":
			v, err := nextValue(args, &i, arg)
			if err != nil {
				return nil, "", err
			}
			if v != "plain" && v != "jsonl" {
				return nil, "", fmt.Errorf("--format must be plain or jsonl")
			}
			opts.format = v
		case strings.HasPrefix(arg, "-"):
			return nil, "", fmt.Errorf("unknown option %q", arg)
		default:
			positional++
			if positional > 1 {
				return nil, "", fmt.Errorf("expected exactly one query argument, got multiple")
			}
			query = arg
		}
	}
done:
	return opts, query, nil
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
func loadConfig(path string) (*config.Config, error) {
	if path != "" {
		return config.Load(path)
	}
	if defaultPath, err := config.DefaultPath(); err == nil {
		if _, statErr := os.Stat(defaultPath); statErr == nil {
			return config.Load(defaultPath)
		}
	}
	return config.Defaults(), nil
}

// runCLISearch executes one search and prints the results. It returns
// the process exit code.
func runCLISearch(cfg *config.Config, opts *cliOptions, query string) int {
	dt := opts.dictionary
	if dt == "" {
		dt = cfg.DefaultDict()
	}

	repo, err := search.OpenRepository(cfg.Database(dt), dt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick: %v\n", err)
		return 2
	}
	defer repo.Close()

	svc, err := search.NewService(repo, cfg.Search.MaxResults)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick: %v\n", err)
		return 2
	}

	entries, err := svc.Search(context.Background(), query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ejquick: %v\n", err)
		return 2
	}
	if len(entries) == 0 {
		return 1
	}

	format := opts.format
	if format == "" {
		format = "plain"
	}
	var wErr error
	if format == "jsonl" {
		wErr = writeJSONL(os.Stdout, dt, entries)
	} else {
		wErr = writePlain(os.Stdout, entries)
	}
	if wErr != nil {
		fmt.Fprintf(os.Stderr, "ejquick: write output: %v\n", wErr)
		return 2
	}
	return 0
}

// writePlain prints headword, body, and a blank line per entry. The last
// entry also ends with a newline.
func writePlain(f *os.File, entries []search.Entry) error {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Headword)
		b.WriteByte('\n')
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	_, err := f.WriteString(b.String())
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
func writeJSONL(f *os.File, dt dictionary.Type, entries []search.Entry) error {
	enc := json.NewEncoder(f)
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
