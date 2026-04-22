package backend

import (
	"strings"
	"testing"

	"github.com/alcxyz/canopy/internal/config"
)

func TestBuildWIQL_EmptyFilter(t *testing.T) {
	q := buildWIQL(config.Filter{}, nil, "")
	want := "SELECT [System.Id] FROM WorkItems ORDER BY [System.ChangedDate] DESC"
	if q != want {
		t.Errorf("got:\n  %s\nwant:\n  %s", q, want)
	}
}

func TestBuildWIQL_Types(t *testing.T) {
	q := buildWIQL(config.Filter{Types: []string{"feature", "bug"}}, nil, "")
	if !strings.Contains(q, "[System.WorkItemType] IN ('Feature', 'Bug')") {
		t.Errorf("expected type clause, got: %s", q)
	}
}

func TestBuildWIQL_Status(t *testing.T) {
	q := buildWIQL(config.Filter{Status: []string{"in-progress", "done"}}, nil, "")
	if !strings.Contains(q, "[System.State] IN (") {
		t.Errorf("expected state clause, got: %s", q)
	}
	// Should contain Azure state names
	if !strings.Contains(q, "'Active'") {
		t.Errorf("expected Active state, got: %s", q)
	}
	if !strings.Contains(q, "'Closed'") {
		t.Errorf("expected Closed state, got: %s", q)
	}
}

func TestBuildWIQL_AssigneeMe(t *testing.T) {
	q := buildWIQL(config.Filter{Assignee: "me"}, nil, "")
	if !strings.Contains(q, "[System.AssignedTo] = @me") {
		t.Errorf("expected @me clause, got: %s", q)
	}
}

func TestBuildWIQL_AssigneeSpecific(t *testing.T) {
	q := buildWIQL(config.Filter{Assignee: "alice@example.com"}, nil, "")
	if !strings.Contains(q, "[System.AssignedTo] = 'alice@example.com'") {
		t.Errorf("expected specific assignee, got: %s", q)
	}
}

func TestBuildWIQL_TeamFilter(t *testing.T) {
	team := []string{"alice@example.com", "bob@example.com"}
	q := buildWIQL(config.Filter{}, team, "")
	if !strings.Contains(q, "[System.AssignedTo] IN ('alice@example.com', 'bob@example.com')") {
		t.Errorf("expected team IN clause, got: %s", q)
	}
}

func TestBuildWIQL_TeamIgnoredWhenAssigneeSet(t *testing.T) {
	team := []string{"alice@example.com"}
	q := buildWIQL(config.Filter{Assignee: "me"}, team, "")
	// Should use @me, not the team IN clause
	if strings.Contains(q, "IN (") {
		t.Errorf("team filter should not be used when assignee is set, got: %s", q)
	}
}

func TestBuildWIQL_UpdatedSince(t *testing.T) {
	q := buildWIQL(config.Filter{UpdatedSince: "last_week"}, nil, "")
	if !strings.Contains(q, "[System.ChangedDate] >= @today - 7") {
		t.Errorf("expected date clause, got: %s", q)
	}
}

func TestBuildWIQL_Sprint(t *testing.T) {
	q := buildWIQL(config.Filter{Sprint: "current"}, nil, "Project\\Sprint 5")
	if !strings.Contains(q, "[System.IterationPath] = 'Project\\Sprint 5'") {
		t.Errorf("expected iteration clause, got: %s", q)
	}
}

func TestBuildWIQL_Labels(t *testing.T) {
	q := buildWIQL(config.Filter{Labels: []string{"frontend", "priority-high"}}, nil, "")
	if !strings.Contains(q, "[System.Tags] CONTAINS 'frontend'") {
		t.Errorf("expected first tag clause, got: %s", q)
	}
	if !strings.Contains(q, "[System.Tags] CONTAINS 'priority-high'") {
		t.Errorf("expected second tag clause, got: %s", q)
	}
}

func TestBuildWIQL_Combined(t *testing.T) {
	q := buildWIQL(config.Filter{
		Types:        []string{"bug"},
		Status:       []string{"in-progress"},
		UpdatedSince: "last_week",
		Assignee:     "me",
	}, nil, "")

	// All clauses joined with AND
	if strings.Count(q, " AND ") != 3 {
		t.Errorf("expected 3 AND clauses, got: %s", q)
	}
}

func TestMapAzureState(t *testing.T) {
	cases := []struct {
		azure string
		want  string
	}{
		{"New", "todo"},
		{"Active", "in-progress"},
		{"Resolved", "in-review"},
		{"Closed", "done"},
		{"Done", "done"},
		{"Removed", "closed"},
		{"Custom", "custom"}, // unknown maps to lowercase
	}
	for _, c := range cases {
		if got := string(mapAzureState(c.azure)); got != c.want {
			t.Errorf("mapAzureState(%q) = %q, want %q", c.azure, got, c.want)
		}
	}
}

func TestMapAzureType(t *testing.T) {
	cases := []struct {
		azure string
		want  string
	}{
		{"Feature", "feature"},
		{"Bug", "bug"},
		{"User Story", "user-story"},
		{"Task", "task"},
		{"Epic", "epic"},
		{"Custom Type", "custom type"}, // unknown maps to lowercase
	}
	for _, c := range cases {
		if got := string(mapAzureType(c.azure)); got != c.want {
			t.Errorf("mapAzureType(%q) = %q, want %q", c.azure, got, c.want)
		}
	}
}

func TestQuote_EscapesSingleQuotes(t *testing.T) {
	got := quote("O'Brien")
	if got != "'O''Brien'" {
		t.Errorf("quote(O'Brien) = %s, want 'O''Brien'", got)
	}
}
