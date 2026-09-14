// Package startup prepares configuration and dictionary services before the
// Qt main window is created.
package startup

import (
	"errors"
	"io/fs"

	"github.com/simosako/ejquick/internal/config"
)

// ConfigCategory is the closed set of configuration startup failures shown by
// the GUI.
type ConfigCategory string

const (
	ConfigPathUnavailable ConfigCategory = "path_unavailable"
	ConfigMissing         ConfigCategory = "missing"
	ConfigUnreadable      ConfigCategory = "unreadable"
	ConfigInvalid         ConfigCategory = "invalid"
	ConfigUnknown         ConfigCategory = "unknown"
)

// ConfigError preserves the failed path and underlying error for logging while
// exposing a stable category to the GUI.
type ConfigError struct {
	Category ConfigCategory
	Path     string
	Err      error
}

func (e *ConfigError) Error() string { return e.Err.Error() }
func (e *ConfigError) Unwrap() error { return e.Err }

// ConfigCategoryOf returns the stable category of err, or ConfigUnknown.
func ConfigCategoryOf(err error) ConfigCategory {
	var configErr *ConfigError
	if errors.As(err, &configErr) {
		return configErr.Category
	}
	return ConfigUnknown
}

// LoadedConfig contains a successfully loaded configuration and the path that
// was considered. Defaulted is true only when the default path did not exist.
type LoadedConfig struct {
	Config    *config.Config
	Path      string
	Defaulted bool
}

// LoadConfig applies the GUI startup rules to an explicit or default path. An
// explicit path must exist; a missing default path uses config.Defaults.
func LoadConfig(path string, explicit bool) (*LoadedConfig, error) {
	return loadConfig(path, explicit, configDependencies{
		defaultPath: config.DefaultPath,
		load:        config.Load,
		defaults:    config.Defaults,
	})
}

type configDependencies struct {
	defaultPath func() (string, error)
	load        func(string) (*config.Config, error)
	defaults    func() *config.Config
}

func loadConfig(path string, explicit bool, deps configDependencies) (*LoadedConfig, error) {
	if !explicit {
		var err error
		path, err = deps.defaultPath()
		if err != nil {
			return nil, &ConfigError{Category: ConfigPathUnavailable, Err: err}
		}
	}

	cfg, err := deps.load(path)
	if err == nil {
		return &LoadedConfig{Config: cfg, Path: path}, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		if !explicit {
			return &LoadedConfig{Config: deps.defaults(), Path: path, Defaulted: true}, nil
		}
		return nil, &ConfigError{Category: ConfigMissing, Path: path, Err: err}
	}

	category := ConfigUnknown
	if loadCategory, ok := config.LoadCategoryOf(err); ok {
		switch loadCategory {
		case config.LoadUnreadable:
			category = ConfigUnreadable
		case config.LoadInvalid:
			category = ConfigInvalid
		}
	}
	return nil, &ConfigError{Category: category, Path: path, Err: err}
}
