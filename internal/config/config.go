package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BackendType identifies which task-tracking system a profile connects to.
type BackendType string

const (
	BackendAzureBoards BackendType = "azure-boards"
	BackendGitHub      BackendType = "github"
	BackendJira        BackendType = "jira"
	BackendLinear      BackendType = "linear"
)

// Filter defines criteria for selecting tasks in a view.
type Filter struct {
	UpdatedSince string   `yaml:"updated_since"` // relative: "last_week", "last_month", etc.
	Types        []string `yaml:"types"`          // feature, bug, user-story, task, etc.
	Status       []string `yaml:"status"`         // done, in-progress, in-review, todo, etc.
	Sprint       string   `yaml:"sprint"`         // "current", "previous", or sprint name
	Assignee     string   `yaml:"assignee"`       // "me", or a team member name/email
	Labels       []string `yaml:"labels"`
}

// View is a named filter preset for meetings and workflows.
type View struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Filters     Filter `yaml:"filters"`
}

// Profile connects to one task backend (one project/org).
type Profile struct {
	Name    string      `yaml:"name"`
	Backend BackendType `yaml:"backend"`

	// Azure Boards
	Org     string `yaml:"org"`
	Project string `yaml:"project"`

	// GitHub Issues
	Owner string   `yaml:"owner"`
	Repos []string `yaml:"repos"`

	// Jira
	URL string `yaml:"url"`

	// Linear
	TeamID string `yaml:"team_id"`

	// Common
	Team []string `yaml:"team"` // team member identifiers
}

// Config is the top-level canopy configuration.
type Config struct {
	Profiles    []Profile `yaml:"profiles"`
	Views       []View    `yaml:"views"`
	RefreshSecs int       `yaml:"refresh_secs"`
}

var Default = Config{
	RefreshSecs: 300,
}

// xdgDir returns the XDG base directory, falling back to the provided default
// relative to $HOME.
func xdgDir(envKey, homeRel string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, homeRel)
}

func xdgConfigPath() string {
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "canopy", "config.yaml")
}

// ConfigPath returns the config file path.
func ConfigPath() string {
	return xdgConfigPath()
}

// LogPath returns the path for the runtime log file.
func LogPath() string {
	dir := filepath.Join(xdgDir("XDG_STATE_HOME", ".local/state"), "canopy")
	return filepath.Join(dir, "canopy.log")
}

// CacheDir returns the XDG cache directory for canopy.
func CacheDir() string {
	return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "canopy")
}

// NeedsBootstrap returns true when no config file exists.
func NeedsBootstrap() bool {
	_, err := os.Stat(xdgConfigPath())
	return err != nil
}

// BootstrapXDG writes the example config to the XDG config path.
func BootstrapXDG(example []byte) (string, error) {
	p := xdgConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(p, example, 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return p, nil
}

// Load reads and parses the config file.
func Load() Config {
	cfg := Default

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	if cfg.RefreshSecs <= 0 {
		cfg.RefreshSecs = Default.RefreshSecs
	}

	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == "" {
			cfg.Profiles[i].Name = fmt.Sprintf("profile %d", i+1)
		}
	}

	return cfg
}

// ViewByName returns the view with the given name, or nil if not found.
func (c Config) ViewByName(name string) *View {
	for i := range c.Views {
		if c.Views[i].Name == name {
			return &c.Views[i]
		}
	}
	return nil
}
