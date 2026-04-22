package backend

import (
	"context"
	"fmt"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type githubBackend struct {
	profile config.Profile
}

func newGitHub(p config.Profile) (Backend, error) {
	if p.Owner == "" {
		return nil, fmt.Errorf("github: owner is required")
	}
	return &githubBackend{profile: p}, nil
}

func (g *githubBackend) Name() string { return g.profile.Name }

func (g *githubBackend) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	// TODO: implement via gh CLI or GitHub API
	return nil, fmt.Errorf("github: not yet implemented")
}

func (g *githubBackend) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	// GitHub doesn't have native sprints; could map milestones.
	return nil, nil
}

func (g *githubBackend) ListTeam(ctx context.Context) ([]model.TeamMember, error) {
	return nil, fmt.Errorf("github: not yet implemented")
}
