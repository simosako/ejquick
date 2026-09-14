// Configuration loading for the GUI, with the closed category set of
// design D46. The dialog text and actions are derived from the category
// alone; raw errors go to the log. A missing default config file is not
// an error: defaults apply, exactly as in the CLI (design D39).
package gui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/simosako/ejquick/internal/config"
)

// ConfigCategory classifies why the configuration could not be loaded
// (design D46). The set is closed.
type ConfigCategory int

const (
	// ConfigPathUnavailable: the default configuration path could not be
	// determined. Never happens with an explicit --config path.
	ConfigPathUnavailable ConfigCategory = iota
	// ConfigMissing: an explicitly given configuration file does not
	// exist.
	ConfigMissing
	// ConfigUnreadable: the file exists but cannot be read.
	ConfigUnreadable
	// ConfigInvalid: TOML parse, unknown key, or validation failure.
	ConfigInvalid
	// ConfigUnknown: anything else.
	ConfigUnknown
)

// ConfigError reports a configuration load failure with its category
// and the configuration path involved.
type ConfigError struct {
	category ConfigCategory
	path     string
	err      error
}

// Error returns the underlying human-readable message.
func (e *ConfigError) Error() string { return e.err.Error() }

// Category returns the load failure classification.
func (e *ConfigError) Category() ConfigCategory { return e.category }

// Path returns the configuration path involved ("" when the default
// path itself could not be determined).
func (e *ConfigError) Path() string { return e.path }

// Unwrap returns the underlying cause. Raw details stay in the log;
// the dialog shows only the category message and the path (design
// D46).
func (e *ConfigError) Unwrap() error { return e.err }

// ConfigMessage returns the canonical dialog message for a category
// (design D46).
func ConfigMessage(c ConfigCategory) string {
	switch c {
	case ConfigPathUnavailable:
		return "The configuration file location could not be determined."
	case ConfigMissing:
		return "The configuration file was not found."
	case ConfigUnreadable:
		return "The configuration file could not be read."
	case ConfigInvalid:
		return "The configuration file is invalid."
	default:
		return "The configuration could not be loaded."
	}
}

// ConfigOpenAction selects which open action the startup dialog offers
// in addition to Retry and Quit (design D46).
type ConfigOpenAction int

const (
	// ConfigOpenNone offers no open action.
	ConfigOpenNone ConfigOpenAction = iota
	// ConfigOpenParentFolder opens the nearest existing ancestor
	// directory of the configuration path.
	ConfigOpenParentFolder
	// ConfigOpenFile opens the configuration file itself.
	ConfigOpenFile
)

// ConfigOpenChoice returns the open action for a category and the
// current file state: a file action only for an existing file, else the
// nearest existing parent folder. "unknown" picks whichever applies
// (design D46).
func ConfigOpenChoice(category ConfigCategory, path string) ConfigOpenAction {
	switch category {
	case ConfigMissing:
		return ConfigOpenParentFolder
	case ConfigUnreadable, ConfigInvalid:
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return ConfigOpenFile
			}
		}
		return ConfigOpenParentFolder
	case ConfigPathUnavailable:
		return ConfigOpenNone
	default: // ConfigUnknown
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return ConfigOpenFile
			}
			return ConfigOpenParentFolder
		}
		return ConfigOpenNone
	}
}

// defaultConfigPath resolves the default configuration path; tests swap
// it to exercise the default-path logic portably.
var defaultConfigPath = config.DefaultPath

// LoadConfig loads the explicit configuration path, or the default one.
// An explicit path must exist and be readable. A missing default file
// means all defaults apply (nil, nil). Every failure is a *ConfigError;
// the error strings match the CLI's.
func LoadConfig(path string, explicit bool) (*config.Config, error) {
	if explicit {
		return loadExplicitConfig(path)
	}
	defaultPath, err := defaultConfigPath()
	if err != nil {
		return nil, &ConfigError{category: ConfigPathUnavailable, err: err}
	}
	return loadDefaultConfigPath(defaultPath)
}

func loadExplicitConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &ConfigError{category: ConfigMissing, path: path, err: err}
		}
		return nil, &ConfigError{category: ConfigUnreadable, path: path, err: err}
	}
	if _, err := os.ReadFile(path); err != nil {
		return nil, &ConfigError{category: ConfigUnreadable, path: path, err: err}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, &ConfigError{category: ConfigInvalid, path: path, err: err}
	}
	return cfg, nil
}

func loadDefaultConfigPath(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return config.Defaults(), nil
		}
		return nil, &ConfigError{category: ConfigUnreadable, path: path, err: err}
	}
	if _, err := os.ReadFile(path); err != nil {
		return nil, &ConfigError{category: ConfigUnreadable, path: path, err: err}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, &ConfigError{category: ConfigInvalid, path: path, err: err}
	}
	return cfg, nil
}

// NearestExistingDir walks up from the configuration path until it
// finds an existing directory, for the Open Parent Folder action
// (design D46). It returns "" when nothing exists up to the root.
func NearestExistingDir(path string) string {
	dir := filepath.Dir(path)
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
