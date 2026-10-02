package app

import (
	"math"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/model"
)

// timeBuckets are the fixed date-range labels for the f date-cycle filter.
var timeBuckets = []string{"today", "yesterday", "this week", "last week", "this month", "last month", "this quarter", "last quarter", "last 6 months", "prior 6 months"}

// dateFields are the available timestamp fields for the F date-field cycle.
var dateFields = []string{"updated", "created", "start", "target", "closed", "state changed"}

// bucketRange returns the [start, end) interval covered by a date bucket.
// Weeks start on Monday.
func bucketRange(label string, now time.Time) (start, end time.Time, ok bool) {
	y, mo, d := now.Date()
	loc := now.Location()
	today := time.Date(y, mo, d, 0, 0, 0, 0, loc)
	tomorrow := today.AddDate(0, 0, 1)
	weekday := int(now.Weekday()+6) % 7 // Monday = 0
	week := today.AddDate(0, 0, -weekday)
	month := time.Date(y, mo, 1, 0, 0, 0, 0, loc)
	quarter := time.Date(y, ((mo-1)/3)*3+1, 1, 0, 0, 0, 0, loc)

	switch label {
	case "today":
		return today, tomorrow, true
	case "yesterday":
		return today.AddDate(0, 0, -1), today, true
	case "this week":
		return week, week.AddDate(0, 0, 7), true
	case "last week":
		return week.AddDate(0, 0, -7), week, true
	case "this month":
		return month, month.AddDate(0, 1, 0), true
	case "last month":
		return month.AddDate(0, -1, 0), month, true
	case "this quarter":
		return quarter, quarter.AddDate(0, 3, 0), true
	case "last quarter":
		return quarter.AddDate(0, -3, 0), quarter, true
	case "last 6 months":
		return today.AddDate(0, -6, 0), tomorrow, true
	case "prior 6 months":
		return today.AddDate(0, -12, 0), today.AddDate(0, -6, 0), true
	}
	return time.Time{}, time.Time{}, false
}

// dateInBucket reports whether t falls within the named time bucket.
func dateInBucket(t time.Time, label string, now time.Time) bool {
	if t.IsZero() {
		return false
	}
	start, end, ok := bucketRange(label, now)
	return ok && !t.Before(start) && t.Before(end)
}

// bucketDays returns how many whole days before today a bucket starts, which
// is the history window a backend query needs to cover it.
func bucketDays(label string, now time.Time) int {
	start, _, ok := bucketRange(label, now)
	if !ok {
		return 0
	}
	y, mo, d := now.Date()
	today := time.Date(y, mo, d, 0, 0, 0, 0, now.Location())
	sy, smo, sd := start.Date()
	startDay := time.Date(sy, smo, sd, 0, 0, 0, 0, now.Location())
	return int(math.Round(today.Sub(startDay).Hours() / 24))
}

// requiredScopeDays returns the history window the task tabs need. The window
// only widens while a date filter is active, so cycling through buckets does
// not reload back and forth, and it is kept while other filters chosen from
// the wider data remain active; clearing all filters restores the default.
// Backends bound queries by last change, which covers every date field except
// the planning dates (start, target); those filter only the tasks loaded.
func (m Model) requiredScopeDays() int {
	label, isDate := m.activeCycleValue("date")
	switch {
	case m.viewOpen():
		return defaultScopeDays
	case isDate:
		return max(m.scopeDays, defaultScopeDays, bucketDays(label, time.Now()))
	case m.hasFilters():
		return max(m.scopeDays, defaultScopeDays)
	}
	return defaultScopeDays
}

// syncScope reloads the task tabs when the date filter needs a different
// history window than the one loaded (ADR-003: f overrides the default scope).
func (m *Model) syncScope() tea.Cmd {
	days := m.requiredScopeDays()
	if days == m.scopeDays || len(m.backends) == 0 {
		return nil
	}
	m.scopeDays = days
	m.statusMsg = "Loading tasks changed in the last " + pluralDays(days) + "…"
	return m.startLoad()
}

// filteredTasks applies text search and cycle filters to a task slice.
func (m Model) filteredTasks(tasks []model.Task) []model.Task {
	q := strings.ToLower(m.filterQuery)
	value, hasCycle := m.activeCycleValue(m.cycleField)
	if q == "" && !hasCycle {
		return tasks
	}
	now := time.Now()
	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if !taskMatchesQuery(t, q) {
			continue
		}
		if hasCycle && !m.cycleMatches(t, value, now) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// cycleMatches reports whether t passes the active cycle filter value.
func (m Model) cycleMatches(t model.Task, value string, now time.Time) bool {
	switch m.cycleField {
	case "assignee":
		return t.Assignee == value
	case "type":
		return string(t.Type) == value
	case "tag":
		return slices.Contains(t.Labels, value)
	case "date":
		return dateInBucket(m.taskDate(t), value, now)
	}
	return true
}

func taskMatchesQuery(t model.Task, q string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(t.Title), q) ||
		strings.Contains(strings.ToLower(t.Assignee), q) ||
		strings.Contains(strings.ToLower(string(t.Type)), q) ||
		strings.Contains(strings.ToLower(t.ID), q) ||
		strings.Contains(strings.ToLower(t.Sprint), q) ||
		labelsContain(t.Labels, q)
}

// labelsContain returns true if any label contains the query substring.
func labelsContain(labels []string, q string) bool {
	for _, l := range labels {
		if strings.Contains(strings.ToLower(l), q) {
			return true
		}
	}
	return false
}

// activeCycleValue returns the selected value when field's cycle filter is active.
func (m Model) activeCycleValue(field string) (string, bool) {
	if field == "" || m.cycleField != field || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return "", false
	}
	return m.cycleValues[m.cycleIdx], true
}

// dateField returns the name of the timestamp used by the date filter.
func (m Model) dateField() string {
	return dateFields[m.dateFieldIdx]
}

// taskDate returns the timestamp from t that corresponds to the active date field.
func (m Model) taskDate(t model.Task) time.Time {
	switch m.dateField() {
	case "created":
		return t.CreatedAt
	case "start":
		return t.StartDate
	case "target":
		return t.TargetDate
	case "closed":
		return t.ClosedAt
	case "state changed":
		return t.StateChangedAt
	default: // "updated"
		return t.UpdatedAt
	}
}

// collectCycleValues gathers unique values for a field from the tasks at the
// current navigation level, after the text filter.
func (m Model) collectCycleValues(field string) []string {
	if field == "date" {
		return timeBuckets
	}

	q := strings.ToLower(m.filterQuery)
	seen := map[string]struct{}{}
	var vals []string
	add := func(v string) {
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		vals = append(vals, v)
	}
	// Seed with config preset tags so they always appear first.
	if field == "tag" {
		for _, t := range m.cfg.Tags {
			add(t)
		}
	}
	presetLen := len(vals)
	for _, t := range m.levelTasks(len(m.navStack)) {
		if !taskMatchesQuery(t, q) {
			continue
		}
		switch field {
		case "assignee":
			add(t.Assignee)
		case "type":
			add(string(t.Type))
		case "tag":
			for _, l := range t.Labels {
				add(l)
			}
		}
	}
	slices.Sort(vals[presetLen:])
	return vals
}

// doCycleFilter advances (or starts) a cycle filter for field.
func (m *Model) doCycleFilter(field string) {
	newVals := m.collectCycleValues(field)
	if len(newVals) == 0 {
		return
	}
	m.cursor = 0
	if m.cycleField != field {
		m.cycleField = field
		m.cycleValues = newVals
		m.cycleIdx = 0
		return
	}

	current, _ := m.activeCycleValue(field)
	nextIdx := 0
	if i := slices.Index(newVals, current); i >= 0 {
		nextIdx = i + 1
	}
	if nextIdx >= len(newVals) {
		m.clearCycleFilter()
		return
	}
	m.cycleValues = newVals
	m.cycleIdx = nextIdx
}

// doCycleDateField advances the date field used by the f date-cycle filter.
func (m *Model) doCycleDateField() {
	m.dateFieldIdx = (m.dateFieldIdx + 1) % len(dateFields)
}

// clearCycleFilter resets any active cycle filter.
func (m *Model) clearCycleFilter() {
	m.cycleField = ""
	m.cycleValues = nil
	m.cycleIdx = -1
}

// hasFilters reports whether a text or cycle filter is active.
func (m Model) hasFilters() bool {
	return m.filterQuery != "" || m.cycleField != ""
}

// clearFilters removes the text and cycle filters.
func (m *Model) clearFilters() {
	m.clearCycleFilter()
	m.filterQuery = ""
	m.cursor = 0
}
