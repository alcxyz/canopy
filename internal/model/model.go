package model

import "time"

// TaskState is the normalised status of a task across all backends.
type TaskState string

const (
	StateTodo       TaskState = "todo"
	StateInProgress TaskState = "in-progress"
	StateInReview   TaskState = "in-review"
	StateDone       TaskState = "done"
	StateClosed     TaskState = "closed"
)

// TaskType is the normalised work-item type across all backends.
type TaskType string

const (
	TypeFeature   TaskType = "feature"
	TypeBug       TaskType = "bug"
	TypeUserStory TaskType = "user-story"
	TypeTask      TaskType = "task"
	TypeEpic      TaskType = "epic"
	TypeSubtask   TaskType = "subtask"
)

// Task is the unified work item across all backends.
type Task struct {
	ID             string
	Title          string
	State          TaskState
	Type           TaskType
	Assignee       string
	Labels         []string
	Sprint         string
	URL            string // web URL to open in browser
	Profile        string // which profile this came from
	Backend        string // backend type for display
	ParentID       string // parent work item ID (if any)
	ParentTitle    string // resolved parent title
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartDate      time.Time
	TargetDate     time.Time
	ClosedAt       time.Time
	StateChangedAt time.Time
}

// Sprint represents an iteration/sprint across backends.
type Sprint struct {
	ID        string
	Name      string
	StartDate time.Time
	EndDate   time.Time
	Profile   string
}

// TeamMember represents a person on the team.
type TeamMember struct {
	ID      string
	Name    string
	Email   string
	Profile string
}
