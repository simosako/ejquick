package startup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// Service is a searchable, closeable dictionary service owned by the GUI.
type Service interface {
	Search(context.Context, string) ([]search.Entry, error)
	Close() error
}

// Logger is the startup subset provided by logging.Logger.
type Logger interface {
	Error(string, ...any)
	Debug(string, ...any)
}

// DatabaseStatus describes one dictionary after synchronous startup checks.
// Err is retained for logging and must not be rendered directly in the GUI.
type DatabaseStatus struct {
	Available bool
	Category  search.OpenCategory
	Err       error
}

// Databases owns the successfully opened services and the status of both
// dictionaries.
type Databases struct {
	Services map[dictionary.Type]Service
	Statuses map[dictionary.Type]DatabaseStatus
	Initial  dictionary.Type
}

// OpenDatabases checks the configured default dictionary first and the other
// dictionary second. A failure never prevents the other database from being
// checked.
func OpenDatabases(cfg *config.Config, logger Logger) *Databases {
	return openDatabases(cfg, logger, openSearchService)
}

type openServiceFunc func(string, dictionary.Type, int) (Service, error)

func openDatabases(cfg *config.Config, logger Logger, openService openServiceFunc) *Databases {
	state := &Databases{
		Services: make(map[dictionary.Type]Service, len(dictionary.All)),
		Statuses: make(map[dictionary.Type]DatabaseStatus, len(dictionary.All)),
		Initial:  cfg.DefaultDict(),
	}
	order := []dictionary.Type{cfg.DefaultDict(), cfg.DefaultDict().Other()}
	for _, dict := range order {
		path := cfg.Database(dict)
		started := time.Now()
		service, err := openService(path, dict, cfg.Search.MaxResults)
		elapsed := time.Since(started).Round(time.Microsecond)
		if err != nil || service == nil {
			if service != nil {
				_ = service.Close()
			}
			if err == nil {
				err = errors.New("open service returned nil")
			}
			category := search.OpenCategoryOf(err)
			state.Statuses[dict] = DatabaseStatus{Category: category, Err: err}
			if logger != nil {
				logger.Error("gui startup: open database dictionary=%s path=%q category=%s: %v", dict, path, category, err)
				logger.Debug("gui startup: open database dictionary=%s available=false elapsed=%s", dict, elapsed)
			}
			continue
		}

		state.Services[dict] = service
		state.Statuses[dict] = DatabaseStatus{Available: true}
		if logger != nil {
			logger.Debug("gui startup: open database dictionary=%s available=true elapsed=%s", dict, elapsed)
		}
	}

	if state.Services[state.Initial] == nil && state.Services[state.Initial.Other()] != nil {
		state.Initial = state.Initial.Other()
	}
	return state
}

func openSearchService(path string, dict dictionary.Type, maxResults int) (Service, error) {
	repository, err := search.OpenRepository(path, dict)
	if err != nil {
		return nil, err
	}
	service, err := search.NewService(repository, maxResults)
	if err != nil {
		_ = repository.Close()
		return nil, fmt.Errorf("create %s search service: %w", dict, err)
	}
	return service, nil
}

// Close releases all successfully opened services. It is idempotent.
func (d *Databases) Close() error {
	if d == nil {
		return nil
	}
	var closeErrors []error
	for _, dict := range dictionary.All {
		service := d.Services[dict]
		if service == nil {
			continue
		}
		delete(d.Services, dict)
		if err := service.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close %s database: %w", dict, err))
		}
	}
	return errors.Join(closeErrors...)
}
