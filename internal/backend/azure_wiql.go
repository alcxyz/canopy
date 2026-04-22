package backend

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// ── State mapping ───────────────────────────────────────────────────────

// canopy → Azure DevOps state names (covers Agile + CMMI process templates).
var stateToAzure = map[model.TaskState][]string{
	model.StateTodo:       {"New", "Proposed"},
	model.StateInProgress: {"Active", "Committed"},
	model.StateInReview:   {"Resolved"},
	model.StateDone:       {"Closed", "Done"},
	model.StateClosed:     {"Closed", "Done", "Removed"},
}

// Azure → canopy (reverse lookup).
var azureToState = map[string]model.TaskState{
	"New":       model.StateTodo,
	"Proposed":  model.StateTodo,
	"Active":    model.StateInProgress,
	"Committed": model.StateInProgress,
	"Resolved":  model.StateInReview,
	"Closed":    model.StateDone,
	"Done":      model.StateDone,
	"Removed":   model.StateClosed,
}

func mapAzureState(s string) model.TaskState {
	if st, ok := azureToState[s]; ok {
		return st
	}
	return model.TaskState(strings.ToLower(s))
}

// ── Type mapping ────────────────────────────────────────────────────────

var typeToAzure = map[model.TaskType]string{
	model.TypeFeature:   "Feature",
	model.TypeBug:       "Bug",
	model.TypeUserStory: "User Story",
	model.TypeTask:      "Task",
	model.TypeEpic:      "Epic",
	model.TypeSubtask:   "Task",
}

var azureToType = map[string]model.TaskType{
	"Feature":    model.TypeFeature,
	"Bug":        model.TypeBug,
	"User Story": model.TypeUserStory,
	"Task":       model.TypeTask,
	"Epic":       model.TypeEpic,
}

func mapAzureType(s string) model.TaskType {
	if t, ok := azureToType[s]; ok {
		return t
	}
	return model.TaskType(strings.ToLower(s))
}

// ── WIQL query builder ──────────────────────────────────────────────────

// buildWIQL constructs a WIQL query from a canopy filter.
// project scopes results to a single Azure DevOps project.
// iterPath is the resolved iteration path (only needed when filter.Sprint == "current").
func buildWIQL(filter config.Filter, project string, team []string, iterPath string) string {
	var clauses []string

	// Always scope to the configured project.
	if project != "" {
		clauses = append(clauses, fmt.Sprintf("[System.TeamProject] = %s", quote(project)))
	}

	// Types
	if len(filter.Types) > 0 {
		var azTypes []string
		for _, t := range filter.Types {
			if az, ok := typeToAzure[model.TaskType(t)]; ok {
				azTypes = append(azTypes, quote(az))
			}
		}
		if len(azTypes) > 0 {
			clauses = append(clauses,
				fmt.Sprintf("[System.WorkItemType] IN (%s)", strings.Join(azTypes, ", ")))
		}
	}

	// Status
	if len(filter.Status) > 0 {
		seen := map[string]bool{}
		var azStates []string
		for _, s := range filter.Status {
			for _, az := range stateToAzure[model.TaskState(s)] {
				if !seen[az] {
					seen[az] = true
					azStates = append(azStates, quote(az))
				}
			}
		}
		if len(azStates) > 0 {
			clauses = append(clauses,
				fmt.Sprintf("[System.State] IN (%s)", strings.Join(azStates, ", ")))
		}
	}

	// Updated since
	if filter.UpdatedSince != "" {
		if days := updatedSinceDays(filter.UpdatedSince); days > 0 {
			clauses = append(clauses,
				fmt.Sprintf("[System.ChangedDate] >= @today - %d", days))
		}
	}

	// Assignee
	if filter.Assignee == "me" {
		clauses = append(clauses, "[System.AssignedTo] = @me")
	} else if filter.Assignee != "" {
		clauses = append(clauses,
			fmt.Sprintf("[System.AssignedTo] = %s", quote(filter.Assignee)))
	} else if len(team) > 0 {
		// No specific assignee but team is configured — filter to team members.
		quoted := make([]string, len(team))
		for i, m := range team {
			quoted[i] = quote(m)
		}
		clauses = append(clauses,
			fmt.Sprintf("[System.AssignedTo] IN (%s)", strings.Join(quoted, ", ")))
	}

	// Sprint / iteration
	if iterPath != "" {
		clauses = append(clauses,
			fmt.Sprintf("[System.IterationPath] = %s", quote(iterPath)))
	} else if filter.Sprint != "" && filter.Sprint != "current" {
		clauses = append(clauses,
			fmt.Sprintf("[System.IterationPath] = %s", quote(filter.Sprint)))
	}

	// Labels / tags
	for _, label := range filter.Labels {
		clauses = append(clauses,
			fmt.Sprintf("[System.Tags] CONTAINS %s", quote(label)))
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	return "SELECT [System.Id] FROM WorkItems" + where + " ORDER BY [System.ChangedDate] DESC"
}

func updatedSinceDays(s string) int {
	switch s {
	case "today":
		return 0
	case "yesterday":
		return 1
	case "last_week":
		return 7
	case "last_2_weeks":
		return 14
	case "last_month":
		return 30
	case "last_quarter":
		return 90
	default:
		// Support "last_N_days" dynamic format.
		if strings.HasPrefix(s, "last_") && strings.HasSuffix(s, "_days") {
			mid := s[len("last_") : len(s)-len("_days")]
			if n, err := strconv.Atoi(mid); err == nil {
				return n
			}
		}
		return 0
	}
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
