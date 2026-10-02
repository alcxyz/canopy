package app

import (
	"encoding/json"
	"fmt"
	"log"
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

var tabNames = []string{"1 My Tasks", "2 Team", "3 Done", "4 Views"}

// defaultScopeDays is how many days of history (by last change) the task tabs
// load by default (ADR-003). The f date filter widens it when a bucket reaches
// further back.
const defaultScopeDays = 7

// Model is the top-level bubbletea model.
type Model struct {
	cfg      config.Config
	backends []backend.Backend

	// Raw data from backends (unfiltered).
	myTasks   []model.Task
	teamTasks []model.Task
	doneTasks []model.Task

	// Views tab: index of the open view (-1 shows the view list) and its tasks.
	viewIdx   int
	viewTasks []model.Task
	viewErr   error // error from the last view load

	// Loading. Each request carries a sequence number so that responses from
	// superseded requests are dropped instead of overwriting newer data.
	loadingTasks  bool
	loadingView   bool
	loadSeq       int
	viewSeq       int
	scopeDays     int       // history window the task tabs are loaded with
	tasksLoadedAt time.Time // when the current task data was fetched

	// UI state
	activeTab     tab
	cursor        int
	offset        int // first visible list row
	width, height int
	ready         bool
	err           error
	statusMsg     string
	notice        string // persistent configuration or backend problems

	// Vim-style gg navigation
	prevKey   string
	prevKeyAt time.Time
	ggTimeout time.Duration

	// Text filter (/ key)
	filtering   bool
	filterQuery string

	// Cycle quick-filter (f=date, d=assignee, s=type, t=tag)
	cycleField  string
	cycleValues []string
	cycleIdx    int

	// Index into dateFields: which timestamp the date filter applies to.
	dateFieldIdx int

	// Navigation stack for drilling into parent tasks.
	navStack []model.Task

	// Overlays
	showHelp   bool
	showSplash bool
	showDetail bool
	detailTask model.Task
	showForm   bool
	form       createForm
	formSeq    int // incremented each time the form opens

	// Cache
	cache *cache.Store

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
	Problems []string // configuration problems to surface to the user
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
	problems := append([]string(nil), o.Problems...)
	var backends []backend.Backend
	for _, p := range o.Cfg.Profiles {
		b, err := backend.New(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("profile %q: %v", p.Name, err))
			continue
		}
		backends = append(backends, b)
	}
	for _, p := range problems {
		log.Print(p)
	}

	m := Model{
		cfg:       o.Cfg,
		backends:  backends,
		viewIdx:   -1,
		cycleIdx:  -1,
		ggTimeout: 400 * time.Millisecond,
		scopeDays: defaultScopeDays,
		notice:    summarize(problems),
		version:   o.Version,
		logPath:   o.LogPath,
		cfgPath:   o.CfgPath,
		cacheDir:  o.CacheDir,
	}
	if m.version == "" {
		m.version = "dev"
	}
	if len(backends) > 0 {
		// Init issues the first load with this sequence number.
		m.loadingTasks = true
		m.loadSeq = 1
	}

	// Initialise cache and load last-known data for instant startup.
	if cs, err := cache.New(o.CacheDir, o.Cfg.CacheKey()); err == nil {
		m.cache = cs
		m.loadCachedTasks()
		st := cs.LoadUIState()
		if st.ActiveTab >= int(tabMyTasks) && st.ActiveTab <= int(tabViews) {
			m.activeTab = tab(st.ActiveTab)
		}
	} else {
		log.Printf("cache: %v", err)
	}

	return m
}

// summarize reduces a list of problems to one status line.
func summarize(problems []string) string {
	switch len(problems) {
	case 0:
		return ""
	case 1:
		return problems[0]
	default:
		return fmt.Sprintf("%s (+%d more, see log)", problems[0], len(problems)-1)
	}
}

// loadCachedTasks restores task lists from the on-disk cache.
// No TTL is enforced here — stale data is shown immediately and replaced
// by a background refresh.
func (m *Model) loadCachedTasks() {
	var cachedAt time.Time
	load := func(key string) []model.Task {
		e := m.cache.Get(key, 0)
		if e == nil {
			return nil
		}
		var ct cachedTasks
		if err := json.Unmarshal(e.Data, &ct); err != nil {
			return nil
		}
		cachedAt = e.CachedAt
		return ct.Tasks
	}
	m.myTasks = load("my_tasks")
	m.teamTasks = load("team_tasks")
	m.doneTasks = load("done_tasks")
	if len(m.myTasks)+len(m.teamTasks)+len(m.doneTasks) > 0 {
		m.tasksLoadedAt = cachedAt
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
		for key, tasks := range map[string][]model.Task{"my_tasks": my, "team_tasks": team, "done_tasks": done} {
			if err := cs.Set(key, cachedTasks{Tasks: tasks}); err != nil {
				log.Printf("cache: saving %s: %v", key, err)
			}
		}
	}()
}

// saveState persists the current UI state (active tab). It is written
// synchronously so the choice survives an immediate quit.
func (m Model) saveState() {
	if m.cache == nil {
		return
	}
	if err := m.cache.SaveUIState(cache.UIState{ActiveTab: int(m.activeTab)}); err != nil {
		log.Printf("cache: saving UI state: %v", err)
	}
}
