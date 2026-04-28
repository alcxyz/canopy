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

// CreateTaskParams holds the inputs for creating a new work item.
type CreateTaskParams struct {
	Type        model.TaskType
	Title       string
	Description string // plain text; backends convert to their native format
	ParentID    string // optional parent work item ID
	Iteration   string // iteration/sprint path; empty = backend default
	Assignee    string // display name or email; empty = unassigned

	DescriptionHTML    string   // pre-formatted HTML; takes precedence over Description when set
	Tags               []string // backend-agnostic labels; Azure joins with "; "
	StartDate          string   // YYYY-MM-DD; empty = not set
	TargetDate         string   // YYYY-MM-DD; empty = not set
	AcceptanceCriteria string   // plain text; backends convert to native format
}

// CreateTaskResult holds the outcome of a successful creation.
type CreateTaskResult struct {
	Task model.Task
}

// TaskCreator is an optional interface for backends that support creating
// work items. Check with: if creator, ok := b.(TaskCreator); ok { ... }
type TaskCreator interface {
	CreateTask(ctx context.Context, params CreateTaskParams) (CreateTaskResult, error)
	// CurrentIteration returns the current sprint/iteration path.
	CurrentIteration(ctx context.Context) (string, error)
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
