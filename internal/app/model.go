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
	tasks []model.Task

	// UI state
	activeTab     tab
	activeProfile int // -1 = all
	activeView    int
	cursor        int
	width, height int
	ready         bool
}

// New creates a new Model from the loaded config.
func New(cfg config.Config) Model {
	var backends []backend.Backend
	for _, p := range cfg.Profiles {
		b, err := backend.New(p)
		if err != nil {
			continue // skip misconfigured profiles
		}
		backends = append(backends, b)
	}
	return Model{
		cfg:      cfg,
		backends: backends,
	}
}
