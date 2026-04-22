package backend

import (
	"context"
	"fmt"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type azureBoards struct {
	profile config.Profile
}

func newAzureBoards(p config.Profile) (Backend, error) {
	if p.Org == "" || p.Project == "" {
		return nil, fmt.Errorf("azure-boards: org and project are required")
	}
	return &azureBoards{profile: p}, nil
}

func (a *azureBoards) Name() string { return a.profile.Name }

func (a *azureBoards) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	// TODO: implement Azure DevOps REST API calls
	return nil, fmt.Errorf("azure-boards: not yet implemented")
}

func (a *azureBoards) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	return nil, fmt.Errorf("azure-boards: not yet implemented")
}

func (a *azureBoards) ListTeam(ctx context.Context) ([]model.TeamMember, error) {
	return nil, fmt.Errorf("azure-boards: not yet implemented")
}
