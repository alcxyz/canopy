package app

import (
	"encoding/json"
	"time"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/cache"
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
	cursor        int
	width, height int
	ready         bool
	loading       bool
	err           error
	statusMsg     string

	// Vim-style gg navigation
	prevKey   string
	prevKeyAt time.Time
	ggTimeout time.Duration

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

	// Date field: which timestamp to use for the date cycle filter.
	dateField    string // current field label, e.g. "updated"
	dateFieldIdx int    // index into dateFields

	// Navigation stack for drilling into parent tasks.
	navStack []model.Task

	// Overlays
	showHelp   bool
	showSplash bool
	showDetail bool
	detailTask model.Task

	// Create-form overlay
	showForm           bool
	formField          int // 0=type, 1=title, 2=description, ...
	formType           int // index into formTypes
	formTitle          string
	formDesc           string
	formTags           string
	formStartDate      string
	formTargetDate     string
	formAcceptCriteria string
	formIteration      string // resolved current iteration path
	formAssignee       string // default assignee display name
	formErr            string
	formSubmitting     bool

	// Cache
	cache         *cache.Store
	tasksLoadedAt time.Time // when the current data was fetched

	// Version update check
	latestVersion string // non-empty when a newer release is available

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

// cachedTasks is the shape persisted in the cache files.
type cachedTasks struct {
	Tasks []model.Task `json:"tasks"`
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

	// Initialise cache and load last-known data for instant startup.
	if cs, err := cache.New(o.CacheDir, o.Cfg.CacheKey()); err == nil {
		m.cache = cs
		m.loadCachedTasks()
	}

	return m
}

// loadCachedTasks restores task lists from the on-disk cache.
// No TTL is enforced here — stale data is shown immediately and replaced
// by a background refresh.
func (m *Model) loadCachedTasks() {
	load := func(key string) []model.Task {
		e, _ := m.cache.Get(key, 0)
		if e == nil {
			return nil
		}
		var ct cachedTasks
		if err := json.Unmarshal(e.Data, &ct); err != nil {
			return nil
		}
		return ct.Tasks
	}
	m.myTasks = load("my_tasks")
	m.teamTasks = load("team_tasks")
	m.doneTasks = load("done_tasks")
	if len(m.myTasks)+len(m.teamTasks)+len(m.doneTasks) > 0 {
		m.statusMsg = "showing cached data…"
	}
}

// saveCachedTasks persists task lists to disk asynchronously.
func (m Model) saveCachedTasks() {
	if m.cache == nil {
		return
	}
	cs := m.cache
	my := m.myTasks
	team := m.teamTasks
	done := m.doneTasks
	go func() {
		_ = cs.Set("my_tasks", cachedTasks{Tasks: my})
		_ = cs.Set("team_tasks", cachedTasks{Tasks: team})
		_ = cs.Set("done_tasks", cachedTasks{Tasks: done})
	}()
}
