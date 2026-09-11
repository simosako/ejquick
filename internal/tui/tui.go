package tui

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// Run starts the TUI. It opens each configured database, verifies it,
// and fails before drawing only when neither dictionary is usable.
func Run(cfg *config.Config, logFile io.Writer) error {
	services := make(map[dictionary.Type]*search.Service)
	unavailable := make(map[dictionary.Type]string)

	for _, dt := range dictionary.All {
		path := cfg.Database(dt)
		if path == "" {
			unavailable[dt] = "not configured"
			continue
		}
		repo, err := search.OpenRepository(path, dt)
		if err != nil {
			unavailable[dt] = err.Error()
			continue
		}
		svc, err := search.NewService(repo, cfg.Search.MaxResults)
		if err != nil {
			repo.Close()
			unavailable[dt] = err.Error()
			continue
		}
		services[dt] = svc
	}

	if len(services) == 0 {
		return fmt.Errorf("no usable dictionary database\n%s\n%s",
			describeUnavailable(unavailable),
			"Create one with: ejquick-build --type <eiji|waei> --input <TXT> --output <sqlite3>")
	}

	initial := cfg.DefaultDict()
	if _, ok := services[initial]; !ok {
		for _, dt := range dictionary.All {
			if _, ok := services[dt]; ok {
				initial = dt
				break
			}
		}
	}

	logErr := func(err error) {
		if logFile != nil {
			fmt.Fprintf(logFile, "search error: %v\n", err)
		}
	}

	m := New(initial, services, unavailable, logErr)
	p := tea.NewProgram(m)
	_, err := p.Run()
	for _, svc := range services {
		svc.Close()
	}
	if err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	_ = os.Stdout
	return nil
}

// describeUnavailable formats per-dictionary reasons for the startup
// error message.
func describeUnavailable(unavailable map[dictionary.Type]string) string {
	var out string
	for _, dt := range dictionary.All {
		if reason, ok := unavailable[dt]; ok {
			out += fmt.Sprintf("  %s: %s\n", dt.Label(), reason)
		}
	}
	return out
}
