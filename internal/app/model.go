package app

import (
	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type tab int

const (
	tabMyTasks tab = iota
	tabTeam
	tabViews
)

var tabNames = []string{"My Tasks", "Team", "Views"}

// Model is the top-level bubbletea model.
type Model struct {
	cfg      config.Config
	backends []backend.Backend

	// Data
	myTasks   []model.Task
	teamTasks []model.Task

	// UI state
	activeTab     tab
	activeProfile int // -1 = all
	activeView    int // selected view index on Views tab
	cursor        int
	width, height int
	ready         bool
	loading       bool
	err           error
	statusMsg     string // transient status message
}

// New creates a new Model from the loaded config.
func New(cfg config.Config) Model {
	var backends []backend.Backend
	var initErrs []string
	for _, p := range cfg.Profiles {
		b, err := backend.New(p)
		if err != nil {
			initErrs = append(initErrs, err.Error())
			continue
		}
		backends = append(backends, b)
	}
	m := Model{
		cfg:      cfg,
		backends: backends,
	}
	if len(initErrs) > 0 && len(backends) == 0 {
		m.statusMsg = "Backend errors: " + initErrs[0]
	}
	return m
}
