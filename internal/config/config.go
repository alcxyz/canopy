package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alcxyz/canopy/internal/model"
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
	Types        []string `yaml:"types"`         // feature, bug, user-story, task, etc.
	Status       []string `yaml:"status"`        // done, in-progress, in-review, todo, etc.
	Sprint       string   `yaml:"sprint"`        // "current", "previous", a sprint name, or a full iteration path
	Assignee     string   `yaml:"assignee"`      // "me", or a team member name/email
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

	// Azure Boards team name (defaults to "{Project} Team")
	AzureTeam string `yaml:"azure_team"`

	// Common
	Team []string `yaml:"team"` // team member identifiers
}

// Config is the top-level canopy configuration.
type Config struct {
	Profiles    []Profile `yaml:"profiles"`
	Views       []View    `yaml:"views"`
	Tags        []string  `yaml:"tags"` // preset tags for the t cycle filter
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

// Load reads and parses the config file. Problems (unreadable or invalid
// YAML, unknown keys, invalid filter values) are returned as messages rather
// than aborting, so the UI can start and report them.
func Load() (Config, []string) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default, nil
		}
		return Default, []string{fmt.Sprintf("reading config: %v", err)}
	}
	return parse(data)
}

func parse(data []byte) (Config, []string) {
	var problems []string
	cfg := Default

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		// Retry leniently so a stray key does not discard the whole config.
		cfg = Default
		if lerr := yaml.Unmarshal(data, &cfg); lerr != nil {
			return Default, []string{"config: " + oneLine(lerr.Error())}
		}
		problems = append(problems, "config: "+oneLine(err.Error()))
	}

	if cfg.RefreshSecs <= 0 {
		cfg.RefreshSecs = Default.RefreshSecs
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == "" {
			cfg.Profiles[i].Name = fmt.Sprintf("profile %d", i+1)
		}
	}

	return cfg, append(problems, cfg.Problems()...)
}

// oneLine collapses a multi-line error message for the status bar.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var (
	validTypes = map[string]bool{
		string(model.TypeFeature): true, string(model.TypeBug): true,
		string(model.TypeUserStory): true, string(model.TypeTask): true,
		string(model.TypeEpic): true, string(model.TypeSubtask): true,
	}
	validStatuses = map[string]bool{
		string(model.StateTodo): true, string(model.StateInProgress): true,
		string(model.StateInReview): true, string(model.StateDone): true,
		string(model.StateClosed): true,
	}
)

// Problems describes view filter values that no backend would understand.
// Such values are otherwise silently ignored, widening the view.
func (c Config) Problems() []string {
	var out []string
	for _, v := range c.Views {
		f := v.Filters
		for _, t := range f.Types {
			if !validTypes[t] {
				out = append(out, fmt.Sprintf("view %q: unknown type %q", v.Name, t))
			}
		}
		for _, s := range f.Status {
			if !validStatuses[s] {
				out = append(out, fmt.Sprintf("view %q: unknown status %q", v.Name, s))
			}
		}
		if f.UpdatedSince != "" {
			if _, ok := UpdatedSinceDays(f.UpdatedSince); !ok {
				out = append(out, fmt.Sprintf("view %q: unknown updated_since %q", v.Name, f.UpdatedSince))
			}
		}
	}
	return out
}

// UpdatedSinceDays converts an updated_since value ("today", "last_week",
// "last_N_days", …) to a number of days back from today.
func UpdatedSinceDays(s string) (int, bool) {
	switch s {
	case "today":
		return 0, true
	case "yesterday":
		return 1, true
	case "last_week":
		return 7, true
	case "last_2_weeks":
		return 14, true
	case "last_month":
		return 30, true
	case "last_quarter":
		return 90, true
	}
	if mid, ok := strings.CutPrefix(s, "last_"); ok {
		if mid, ok = strings.CutSuffix(mid, "_days"); ok {
			if n, err := strconv.Atoi(mid); err == nil && n >= 0 {
				return n, true
			}
		}
	}
	return 0, false
}

// CacheKey returns a string derived from profile settings so the cache is
// automatically invalidated when the user changes org/project/owner.
func (c Config) CacheKey() string {
	var parts []string
	for _, p := range c.Profiles {
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%s", p.Backend, p.Org, p.Project, p.Owner))
	}
	return strings.Join(parts, ";")
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
