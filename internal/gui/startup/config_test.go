package startup

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/simosako/ejquick/internal/config"
)

func TestLoadConfigCategories(t *testing.T) {
	sentinel := errors.New("sentinel")
	tests := []struct {
		name       string
		explicit   bool
		defaultErr error
		loadErr    error
		want       ConfigCategory
		wantPath   string
	}{
		{name: "default path unavailable", defaultErr: sentinel, want: ConfigPathUnavailable},
		{name: "explicit missing", explicit: true, loadErr: fs.ErrNotExist, want: ConfigMissing, wantPath: "custom.toml"},
		{name: "unreadable", explicit: true, loadErr: &config.LoadError{Category: config.LoadUnreadable, Err: sentinel}, want: ConfigUnreadable, wantPath: "custom.toml"},
		{name: "invalid", explicit: true, loadErr: &config.LoadError{Category: config.LoadInvalid, Err: sentinel}, want: ConfigInvalid, wantPath: "custom.toml"},
		{name: "unknown", explicit: true, loadErr: sentinel, want: ConfigUnknown, wantPath: "custom.toml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig("custom.toml", tt.explicit, configDependencies{
				defaultPath: func() (string, error) { return "default.toml", tt.defaultErr },
				load:        func(string) (*config.Config, error) { return nil, tt.loadErr },
				defaults:    config.Defaults,
			})
			if err == nil {
				t.Fatal("loadConfig unexpectedly succeeded")
			}
			if got := ConfigCategoryOf(err); got != tt.want {
				t.Errorf("category = %q, want %q", got, tt.want)
			}
			var configErr *ConfigError
			if !errors.As(err, &configErr) || configErr.Path != tt.wantPath {
				t.Errorf("ConfigError = %#v, want path %q", configErr, tt.wantPath)
			}
		})
	}
}

func TestLoadConfigUsesDefaultsOnlyForMissingDefaultPath(t *testing.T) {
	defaults := &config.Config{}
	defaultCalls := 0
	loaded, err := loadConfig("ignored", false, configDependencies{
		defaultPath: func() (string, error) { return "default.toml", nil },
		load: func(path string) (*config.Config, error) {
			if path != "default.toml" {
				t.Errorf("load path = %q", path)
			}
			return nil, &config.LoadError{Category: config.LoadUnreadable, Err: fs.ErrNotExist}
		},
		defaults: func() *config.Config {
			defaultCalls++
			return defaults
		},
	})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.Config != defaults || loaded.Path != "default.toml" || !loaded.Defaulted || defaultCalls != 1 {
		t.Fatalf("loaded = %#v, default calls = %d", loaded, defaultCalls)
	}
}

func TestLoadConfigReturnsLoadedExplicitConfig(t *testing.T) {
	want := &config.Config{}
	loaded, err := loadConfig("custom.toml", true, configDependencies{
		defaultPath: func() (string, error) { t.Fatal("defaultPath called"); return "", nil },
		load:        func(string) (*config.Config, error) { return want, nil },
		defaults:    func() *config.Config { t.Fatal("defaults called"); return nil },
	})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if loaded.Config != want || loaded.Path != "custom.toml" || loaded.Defaulted {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestMessageForConfig(t *testing.T) {
	tests := []struct {
		category      ConfigCategory
		pathAvailable bool
		pathExists    bool
		logAvailable  bool
		text          string
		open          ConfigOpenTarget
		openLog       bool
	}{
		{ConfigPathUnavailable, false, false, true, "The configuration file location could not be determined.", ConfigOpenNone, true},
		{ConfigMissing, true, false, true, "The configuration file was not found.", ConfigOpenParent, false},
		{ConfigUnreadable, true, true, true, "The configuration file could not be read.", ConfigOpenParent, true},
		{ConfigInvalid, true, true, true, "The configuration file is invalid.", ConfigOpenFile, true},
		{ConfigInvalid, true, false, false, "The configuration file is invalid.", ConfigOpenParent, false},
		{ConfigCategory("future"), true, true, true, "The configuration could not be loaded.", ConfigOpenFile, true},
	}
	for _, tt := range tests {
		got := MessageForConfig(tt.category, tt.pathAvailable, tt.pathExists, tt.logAvailable)
		if got.Text != tt.text || got.OpenTarget != tt.open || got.OpenLog != tt.openLog || !got.Retry || !got.Quit {
			t.Errorf("MessageForConfig(%q) = %#v", tt.category, got)
		}
	}
}
