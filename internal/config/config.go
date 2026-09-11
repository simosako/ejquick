// Package config loads the EJQuick TOML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/simosako/ejquick/internal/dictionary"
	"github.com/simosako/ejquick/internal/search"
)

// Dictionary holds per-dictionary settings.
type Dictionary struct {
	// Database is the path to the SQLite database file.
	Database string `toml:"database"`
}

// Search holds search behavior settings.
type Search struct {
	// DefaultDictionary is "eiji" or "waei".
	DefaultDictionary string `toml:"default_dictionary"`
	// MaxResults is the result limit, 1..500. Zero means the default.
	MaxResults int `toml:"max_results"`
}

// Config is the whole configuration file.
type Config struct {
	Eiji   Dictionary `toml:"eiji"`
	Waei   Dictionary `toml:"waei"`
	Search Search     `toml:"search"`
}

// DefaultPath returns the default configuration file path:
// os.UserConfigDir()/ejquick/config.toml.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config directory: %w", err)
	}
	return filepath.Join(dir, "ejquick", "config.toml"), nil
}

// Load reads and validates the configuration file at path. The file must
// exist and be readable; unknown keys, bad values, and unknown
// dictionaries are startup errors and are not silently corrected.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	cfg.applyDefaults(path)
	return &cfg, nil
}

// LoadDefault reads the configuration from DefaultPath.
func LoadDefault() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// Defaults returns a configuration with every value defaulted, used when
// no config file exists. The base directory argument only affects
// relative configured paths, which defaults never produce.
func Defaults() *Config {
	c := &Config{}
	c.applyDefaults(string(filepath.Separator))
	return c
}

// validate checks all values that must not be silently adjusted.
func (c *Config) validate() error {
	if c.Search.DefaultDictionary != "" {
		if _, err := dictionary.ParseType(c.Search.DefaultDictionary); err != nil {
			return fmt.Errorf("search.default_dictionary: %w", err)
		}
	}
	if c.Search.MaxResults < 0 || c.Search.MaxResults > search.HardMaxResults {
		return fmt.Errorf("search.max_results %d out of range 1..%d",
			c.Search.MaxResults, search.HardMaxResults)
	}
	if c.Eiji.Database != "" && !filepath.IsAbs(expandHome(c.Eiji.Database)) {
		// Relative paths are allowed; resolved against the config
		// directory later. Nothing to reject here.
		_ = c.Eiji.Database
	}
	return nil
}

// applyDefaults fills unset values.
func (c *Config) applyDefaults(configPath string) {
	if c.Search.DefaultDictionary == "" {
		c.Search.DefaultDictionary = dictionary.Eiji.String()
	}
	if c.Search.MaxResults == 0 {
		c.Search.MaxResults = search.DefaultMaxResults
	}
	base := filepath.Dir(configPath)
	if c.Eiji.Database == "" {
		c.Eiji.Database = DefaultDatabasePath(dictionary.Eiji)
	} else {
		c.Eiji.Database = resolvePath(base, c.Eiji.Database)
	}
	if c.Waei.Database == "" {
		c.Waei.Database = DefaultDatabasePath(dictionary.Waei)
	} else {
		c.Waei.Database = resolvePath(base, c.Waei.Database)
	}
}

// DefaultDatabasePath returns the default database location for a
// dictionary when the config file does not set one:
//
//	Linux/macOS: $XDG_DATA_HOME/ejquick or ~/.local/share/ejquick
//	Windows:     %LocalAppData%/ejquick
func DefaultDatabasePath(dt dictionary.Type) string {
	var dir string
	switch dataHome := os.Getenv("XDG_DATA_HOME"); runtime.GOOS {
	case "windows":
		local := os.Getenv("LocalAppData")
		if local == "" {
			local = filepath.Join(os.Getenv("AppData"))
		}
		dir = filepath.Join(local, "ejquick")
	case "darwin", "linux":
		if dataHome != "" {
			dir = filepath.Join(dataHome, "ejquick")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return dt.String() + ".sqlite3"
			}
			dir = filepath.Join(home, ".local", "share", "ejquick")
		}
	default:
		if dataHome != "" {
			dir = filepath.Join(dataHome, "ejquick")
		} else {
			return dt.String() + ".sqlite3"
		}
	}
	return filepath.Join(dir, dt.String()+".sqlite3")
}

// Database returns the configured database path for a dictionary type.
func (c *Config) Database(dt dictionary.Type) string {
	switch dt {
	case dictionary.Eiji:
		return c.Eiji.Database
	case dictionary.Waei:
		return c.Waei.Database
	}
	return ""
}

// DefaultDict returns the configured default dictionary type. The value
// has been validated by Load.
func (c *Config) DefaultDict() dictionary.Type {
	dt, err := dictionary.ParseType(c.Search.DefaultDictionary)
	if err != nil {
		// Unreachable after validate().
		return dictionary.Eiji
	}
	return dt
}

// resolvePath turns a configured path into an absolute one: ~ is expanded
// and relative paths are resolved against the config file directory. An
// empty path stays empty (the dictionary is simply not configured).
func resolvePath(base, p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
				return filepath.Join(home, p[2:])
			}
		}
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(base, p))
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// expandHome expands a leading ~ for validation use only.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
