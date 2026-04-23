package app

import (
	"sort"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/model"
)

// timeBuckets are the fixed date-range labels for the f date-cycle filter.
var timeBuckets = []string{"today", "yesterday", "this week", "last week", "this month", "last month", "this quarter", "last quarter", "last 6 months", "prior 6 months"}

// dateFields are the available timestamp fields for the F date-field cycle.
var dateFields = []string{"updated", "created", "start", "target", "closed", "state changed"}

// dateInBucket returns true if t falls within the named time bucket.
func dateInBucket(t time.Time, label string) bool {
	if t.IsZero() {
		return false
	}
	now := time.Now()
	y, mo, d := now.Date()
	loc := now.Location()
	todayStart := time.Date(y, mo, d, 0, 0, 0, 0, loc)
	switch label {
	case "today":
		return !t.Before(todayStart)
	case "yesterday":
		return !t.Before(todayStart.AddDate(0, 0, -1)) && t.Before(todayStart)
	case "this week":
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		return !t.Before(todayStart.AddDate(0, 0, -(wd - 1)))
	case "last week":
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		thisWeek := todayStart.AddDate(0, 0, -(wd - 1))
		return !t.Before(thisWeek.AddDate(0, 0, -7)) && t.Before(thisWeek)
	case "this month":
		return !t.Before(time.Date(y, mo, 1, 0, 0, 0, 0, loc))
	case "last month":
		thisMonth := time.Date(y, mo, 1, 0, 0, 0, 0, loc)
		return !t.Before(thisMonth.AddDate(0, -1, 0)) && t.Before(thisMonth)
	case "this quarter":
		qStart := time.Date(y, ((mo-1)/3)*3+1, 1, 0, 0, 0, 0, loc)
		return !t.Before(qStart)
	case "last quarter":
		qStart := time.Date(y, ((mo-1)/3)*3+1, 1, 0, 0, 0, 0, loc)
		lastQStart := qStart.AddDate(0, -3, 0)
		return !t.Before(lastQStart) && t.Before(qStart)
	case "last 6 months":
		sixAgo := now.AddDate(0, -6, 0)
		return !t.Before(sixAgo)
	case "prior 6 months":
		sixAgo := now.AddDate(0, -6, 0)
		twelveAgo := now.AddDate(0, -12, 0)
		return !t.Before(twelveAgo) && t.Before(sixAgo)
	}
	return false
}

// dateScopeDays maps a dateScope label to a number of days for the backend
// query's updated_since filter. Returns 0 if no scoping should be applied.
func dateScopeDays(scope string) int {
	switch scope {
	case "today":
		return 1
	case "yesterday":
		return 2
	case "this week":
		return 7
	case "last week":
		return 14
	case "this month":
		return 30
	case "last month":
		return 60
	case "this quarter":
		return 90
	case "last quarter":
		return 180
	case "last 6 months":
		return 180
	case "prior 6 months":
		return 365
	}
	return 7 // default to a week
}

// filteredTasks applies text search and cycle filters to a task slice.
func (m Model) filteredTasks(tasks []model.Task) []model.Task {
	q := strings.ToLower(m.filterQuery)
	hasCycle := m.cycleField != "" && m.cycleIdx >= 0
	if q == "" && !hasCycle {
		return tasks
	}
	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if q != "" {
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Assignee), q) &&
				!strings.Contains(strings.ToLower(string(t.Type)), q) &&
				!strings.Contains(strings.ToLower(t.ID), q) &&
				!strings.Contains(strings.ToLower(t.Sprint), q) &&
				!labelsContain(t.Labels, q) {
				continue
			}
		}
		if !m.cycleMatch("assignee", t.Assignee) {
			continue
		}
		if !m.cycleMatch("type", string(t.Type)) {
			continue
		}
		if !m.cycleMatchTag(t.Labels) {
			continue
		}
		if !m.cycleMatchDate(t) {
			continue
		}
		out = append(out, t)
	}
	return out
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

// cycleMatchTag returns true if any label matches the active tag cycle filter.
func (m Model) cycleMatchTag(labels []string) bool {
	if m.cycleField != "tag" || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	target := m.cycleValues[m.cycleIdx]
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// cycleMatch returns true if item passes the active cycle filter for the given field.
func (m Model) cycleMatch(field, value string) bool {
	if m.cycleField != field || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	return value == m.cycleValues[m.cycleIdx]
}

// taskDateField returns the timestamp from t that corresponds to the active date field.
func (m Model) taskDateField(t model.Task) time.Time {
	switch m.dateField {
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

// cycleMatchDate returns true if the task's active date field falls in the active date bucket.
func (m Model) cycleMatchDate(t model.Task) bool {
	if m.cycleField != "date" || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	return dateInBucket(m.taskDateField(t), m.cycleValues[m.cycleIdx])
}

// collectCycleValues gathers unique sorted values for a field from the
// currently visible tasks on the active tab.
func (m Model) collectCycleValues(field string) []string {
	if field == "date" {
		return timeBuckets
	}
	var tasks []model.Task
	switch m.activeTab {
	case tabMyTasks:
		tasks = m.myTasks
	case tabTeam:
		tasks = m.teamTasks
	case tabDone:
		tasks = m.doneTasks
	default:
		return nil
	}

	// Apply text filter only (not cycle) so we collect values from the text-filtered set.
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
	for _, t := range tasks {
		if q != "" {
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Assignee), q) &&
				!strings.Contains(strings.ToLower(string(t.Type)), q) &&
				!labelsContain(t.Labels, q) {
				continue
			}
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
	sort.Strings(vals)
	return vals
}

// doCycleFilter advances (or starts) a cycle filter for field.
func (m *Model) doCycleFilter(field string) {
	newVals := m.collectCycleValues(field)
	if len(newVals) == 0 {
		return
	}
	if m.cycleField != field {
		m.cycleField = field
		m.cycleValues = newVals
		m.cycleIdx = 0
	} else {
		currentVal := ""
		if m.cycleIdx >= 0 && m.cycleIdx < len(m.cycleValues) {
			currentVal = m.cycleValues[m.cycleIdx]
		}
		nextIdx := 0
		for i, v := range newVals {
			if v == currentVal {
				nextIdx = i + 1
				break
			}
		}
		m.cycleValues = newVals
		if nextIdx >= len(newVals) {
			m.clearCycleFilter()
			return
		}
		m.cycleIdx = nextIdx
	}
	m.cursor = 0
}

// doCycleDateField advances the date field used by the f date-cycle filter.
func (m *Model) doCycleDateField() {
	m.dateFieldIdx = (m.dateFieldIdx + 1) % len(dateFields)
	m.dateField = dateFields[m.dateFieldIdx]
}

// clearCycleFilter resets any active cycle filter.
func (m *Model) clearCycleFilter() {
	m.cycleField = ""
	m.cycleValues = nil
	m.cycleIdx = -1
}
