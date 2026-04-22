package backend

import (
	"context"
	"fmt"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type linearBackend struct {
	profile config.Profile
}

func newLinear(p config.Profile) (Backend, error) {
	if p.TeamID == "" {
		return nil, fmt.Errorf("linear: team_id is required")
	}
	return &linearBackend{profile: p}, nil
}

func (l *linearBackend) Name() string { return l.profile.Name }

func (l *linearBackend) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	// TODO: implement Linear GraphQL API calls
	return nil, fmt.Errorf("linear: not yet implemented")
}

func (l *linearBackend) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	// Linear uses cycles instead of sprints.
	return nil, fmt.Errorf("linear: not yet implemented")
}

func (l *linearBackend) ListTeam(ctx context.Context) ([]model.TeamMember, error) {
	return nil, fmt.Errorf("linear: not yet implemented")
}
