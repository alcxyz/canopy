package backend

import (
	"context"
	"fmt"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type jiraBackend struct {
	profile config.Profile
}

func newJira(p config.Profile) (Backend, error) {
	if p.URL == "" || p.Project == "" {
		return nil, fmt.Errorf("jira: url and project are required")
	}
	return &jiraBackend{profile: p}, nil
}

func (j *jiraBackend) Name() string { return j.profile.Name }

func (j *jiraBackend) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	// TODO: implement Jira REST API calls
	return nil, fmt.Errorf("jira: not yet implemented")
}

func (j *jiraBackend) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	return nil, fmt.Errorf("jira: not yet implemented")
}

func (j *jiraBackend) ListTeam(ctx context.Context) ([]model.TeamMember, error) {
	return nil, fmt.Errorf("jira: not yet implemented")
}
