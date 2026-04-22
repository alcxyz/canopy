package app

import (
	"time"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type tab int

const (
	tabMyTasks tab = iota
	tabTeam
	tabDone
	tabViews
)

var tabNames = []string{"My Tasks [1]", "Team [2]", "Done [3]", "Views [4]"}

// Model is the top-level bubbletea model.
type Model struct {
	cfg      config.Config
	backends []backend.Backend

	// Raw data from backends (unfiltered).
	myTasks   []model.Task
	teamTasks []model.Task
	doneTasks []model.Task

	// UI state
	activeTab     tab
	activeView    int // selected view index on Views tab
	cursor        int
	width, height int
	ready         bool
	loading       bool
	err           error
	statusMsg     string

	// Vim-style gg navigation
	prevKey    string
	prevKeyAt  time.Time
	ggTimeout  time.Duration

	// Text filter (/ key)
	filtering   bool
	filterQuery string

	// Cycle quick-filter (f=date, d=assignee, s=type)
	cycleField  string
	cycleValues []string
	cycleIdx    int

	// Date scope: the default time range applied to backend queries.
	// Overridden by the f cycle filter for client-side filtering.
	dateScope string // "this week" by default

	// Overlays
	showHelp   bool
	showSplash bool

	// Paths shown in splash
	version  string
	logPath  string
	cfgPath  string
	cacheDir string
}

// Options holds the parameters for creating a new Model.
type Options struct {
	Cfg      config.Config
	Version  string
	LogPath  string
	CfgPath  string
	CacheDir string
}

// New creates a new Model from the loaded config.
func New(o Options) Model {
	var backends []backend.Backend
	var initErrs []string
	for _, p := range o.Cfg.Profiles {
		b, err := backend.New(p)
		if err != nil {
			initErrs = append(initErrs, err.Error())
			continue
		}
		backends = append(backends, b)
	}
	m := Model{
		cfg:       o.Cfg,
		backends:  backends,
		cycleIdx:  -1,
		ggTimeout: 400 * time.Millisecond,
		dateScope: "this week",
		version:   o.Version,
		logPath:   o.LogPath,
		cfgPath:   o.CfgPath,
		cacheDir:  o.CacheDir,
	}
	if m.version == "" {
		m.version = "dev"
	}
	if len(initErrs) > 0 && len(backends) == 0 {
		m.statusMsg = "Backend errors: " + initErrs[0]
	}
	return m
}
