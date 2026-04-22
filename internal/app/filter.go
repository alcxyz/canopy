package app

import (
	"sort"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/model"
)

// timeBuckets are the fixed date-range labels for the f date-cycle filter.
var timeBuckets = []string{"this week", "last week", "this month", "last month", "this quarter", "last quarter"}

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
	}
	return false
}

// dateScopeDays maps a dateScope label to a number of days for the backend
// query's updated_since filter. Returns 0 if no scoping should be applied.
func dateScopeDays(scope string) int {
	switch scope {
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
				!strings.Contains(strings.ToLower(t.Sprint), q) {
				continue
			}
		}
		if !m.cycleMatch("assignee", t.Assignee) {
			continue
		}
		if !m.cycleMatch("type", string(t.Type)) {
			continue
		}
		if !m.cycleMatchDate(t.UpdatedAt) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// cycleMatch returns true if item passes the active cycle filter for the given field.
func (m Model) cycleMatch(field, value string) bool {
	if m.cycleField != field || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	return value == m.cycleValues[m.cycleIdx]
}

// cycleMatchDate returns true if t falls in the active date bucket when date cycling.
func (m Model) cycleMatchDate(t time.Time) bool {
	if m.cycleField != "date" || m.cycleIdx < 0 || m.cycleIdx >= len(m.cycleValues) {
		return true
	}
	return dateInBucket(t, m.cycleValues[m.cycleIdx])
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
	for _, t := range tasks {
		if q != "" {
			if !strings.Contains(strings.ToLower(t.Title), q) &&
				!strings.Contains(strings.ToLower(t.Assignee), q) &&
				!strings.Contains(strings.ToLower(string(t.Type)), q) {
				continue
			}
		}
		switch field {
		case "assignee":
			add(t.Assignee)
		case "type":
			add(string(t.Type))
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

// clearCycleFilter resets any active cycle filter.
func (m *Model) clearCycleFilter() {
	m.cycleField = ""
	m.cycleValues = nil
	m.cycleIdx = -1
}
