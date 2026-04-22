package backend

import (
	"context"
	"fmt"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// Backend is the interface that all task-tracking providers implement.
type Backend interface {
	// Name returns a human-readable name for this backend instance.
	Name() string

	// ListTasks returns all tasks matching the given filter.
	ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error)

	// ListSprints returns available sprints/iterations.
	ListSprints(ctx context.Context) ([]model.Sprint, error)

	// ListTeam returns team members for this backend.
	ListTeam(ctx context.Context) ([]model.TeamMember, error)
}

// New creates a Backend from a profile configuration.
func New(profile config.Profile) (Backend, error) {
	switch profile.Backend {
	case config.BackendAzureBoards:
		return newAzureBoards(profile)
	case config.BackendGitHub:
		return newGitHub(profile)
	case config.BackendJira:
		return newJira(profile)
	case config.BackendLinear:
		return newLinear(profile)
	default:
		return nil, fmt.Errorf("unknown backend: %q", profile.Backend)
	}
}
