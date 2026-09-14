// Dictionary startup for the GUI (design D39/D40): both databases are
// opened and validated synchronously before the main window is shown,
// the configured default dictionary first. Failures are per-dictionary
// typed statuses; a valid dictionary never blocks the other one.
package gui

import (
	"time"

	"github.com/simosako/ejquick/internal/config"
	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/logging"
	"github.com/simosako/ejquick/internal/search"
)

// openRepository is the production repository opener; tests substitute
// it to exercise classification without building databases.
type repositoryOpener func(path string, dt dictionary.Type) (*search.Repository, error)

// startupResult is the completed dictionary startup state.
type startupResult struct {
	// Services holds one service per available dictionary.
	Services map[dictionary.Type]Searcher
	// Statuses holds the status of every dictionary.
	Statuses map[dictionary.Type]DictStatus
	// Initial is the dictionary to select first.
	Initial dictionary.Type
}

// openDictionaries opens both configured dictionary databases (design
// D40-C). The default dictionary is validated first, then the other one;
// one failure never skips the other. Raw errors (with the configured
// path) are logged once here (design D44); the view only ever sees the
// category.
func openDictionaries(cfg *config.Config, logger *logging.Logger) startupResult {
	return openDictionariesWith(cfg, logger, search.OpenRepository)
}

func openDictionariesWith(cfg *config.Config, logger *logging.Logger, open repositoryOpener) startupResult {
	if logger == nil {
		logger = logging.Nop()
	}
	res := startupResult{
		Services: map[dictionary.Type]Searcher{},
		Statuses: map[dictionary.Type]DictStatus{},
		Initial:  cfg.DefaultDict(),
	}

	order := []dictionary.Type{cfg.DefaultDict()}
	if other := cfg.DefaultDict().Other(); other != order[0] {
		order = append(order, other)
	}
	for _, dt := range order {
		path := cfg.Database(dt)
		start := time.Now()
		repo, err := open(path, dt)
		elapsed := time.Since(start).Round(time.Microsecond)
		if err != nil {
			category, ok := search.OpenCategoryOf(err)
			if !ok {
				category = search.OpenUnknown
			}
			res.Statuses[dt] = DictStatus{Available: false, Category: category}
			// The raw error and the configured path stay in the log;
			// the user-visible message is derived from the category
			// only (design D44/D45).
			logger.Error("startup: open %s (%s): %v", dt, path, err)
			continue
		}
		svc, err := search.NewService(repo, cfg.Search.MaxResults)
		if err != nil {
			repo.Close()
			res.Statuses[dt] = DictStatus{Available: false, Category: search.OpenUnknown}
			logger.Error("startup: service %s: %v", dt, err)
			continue
		}
		res.Services[dt] = svc
		res.Statuses[dt] = DictStatus{Available: true}
		logger.Debug("startup: opened %s (%s) in %s", dt, path, elapsed)
	}

	res.Initial = initialDictionary(cfg.DefaultDict(), res.Services)
	return res
}

// initialDictionary picks the dictionary to select at startup: the
// configured default when it is available, otherwise the first available
// dictionary in stable order. With no available dictionary the default
// is kept (both are unavailable; the message page explains both).
func initialDictionary(def dictionary.Type, services map[dictionary.Type]Searcher) dictionary.Type {
	if _, ok := services[def]; ok {
		return def
	}
	for _, dt := range dictionary.All {
		if _, ok := services[dt]; ok {
			return dt
		}
	}
	return def
}
