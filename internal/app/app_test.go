package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// fakeBackend returns canned tasks and records the filters it was queried with.
type fakeBackend struct {
	name    string
	tasks   []model.Task
	err     error
	filters chan config.Filter
}

func (f *fakeBackend) Name() string { return f.name }

func (f *fakeBackend) ListTasks(_ context.Context, filter config.Filter) ([]model.Task, error) {
	if f.filters != nil {
		f.filters <- filter
	}
	return f.tasks, f.err
}

func (f *fakeBackend) ListSprints(context.Context) ([]model.Sprint, error)  { return nil, nil }
func (f *fakeBackend) ListTeam(context.Context) ([]model.TeamMember, error) { return nil, nil }
func (f *fakeBackend) CurrentIteration(context.Context) (string, error)     { return "", nil }
func (f *fakeBackend) CreateTask(context.Context, backend.CreateTaskParams) (backend.CreateTaskResult, error) {
	return backend.CreateTaskResult{}, nil
}

func newTestModel(backends ...backend.Backend) Model {
	return Model{
		cfg:        config.Config{RefreshSecs: 300, Views: []config.View{{Name: "Standup"}, {Name: "Review"}}},
		backends:   backends,
		viewIdx:    -1,
		cycleIdx:   -1,
		scopeDays:  defaultScopeDays,
		loadedDays: defaultScopeDays,
		ggTimeout:  400 * time.Millisecond,
		version:    "dev",
		width:      160,
		height:     40,
		ready:      true,
	}
}

func tasks(profile string, n int) []model.Task {
	out := make([]model.Task, n)
	for i := range out {
		out[i] = model.Task{ID: fmt.Sprintf("%s-%d", profile, i), Title: fmt.Sprintf("Task %d", i), Profile: profile}
	}
	return out
}

func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, c := m.Update(msg)
		m, cmd = next.(Model), c
	}
	return m, cmd
}

func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestOpenViewKeepsTaskTabsAndShowsResults(t *testing.T) {
	b := &fakeBackend{name: "Work", tasks: tasks("Work", 3)}
	m := newTestModel(b)
	m.myTasks = tasks("Work", 1)
	m.doneTasks = tasks("Work", 2)
	m.activeTab = tabViews

	m, cmd := press(t, m, "enter")
	if !m.viewOpen() || cmd == nil {
		t.Fatalf("enter on a view should open it and load tasks")
	}
	m = send(m, cmd())

	if len(m.viewTasks) != 3 || len(m.currentTasks()) != 3 {
		t.Errorf("view tasks = %d, current = %d, want 3", len(m.viewTasks), len(m.currentTasks()))
	}
	if len(m.myTasks) != 1 || len(m.doneTasks) != 2 {
		t.Errorf("view load replaced tab data: my=%d done=%d", len(m.myTasks), len(m.doneTasks))
	}
	if !strings.Contains(m.View(), "Standup") {
		t.Error("breadcrumb should name the open view")
	}

	m, _ = press(t, m, "esc")
	if m.viewOpen() || m.cursor != 0 {
		t.Errorf("esc should return to the view list at the view, open=%v cursor=%d", m.viewOpen(), m.cursor)
	}
}

func TestClosedViewIgnoresLateResult(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "Work", tasks: tasks("Work", 3)})
	m.activeTab = tabViews
	m, cmd := press(t, m, "enter")
	m, _ = press(t, m, "esc")
	m = send(m, cmd())
	if len(m.viewTasks) != 0 || m.loadingView {
		t.Error("result for a closed view should be dropped")
	}
}

func TestSupersededLoadIsIgnored(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "Work"})
	m.startLoad()
	m.startLoad()

	m = send(m, tasksLoadedMsg{seq: m.loadSeq - 1, myTasks: tasks("Work", 5)})
	if len(m.myTasks) != 0 || !m.loadingTasks {
		t.Fatal("stale response should be ignored")
	}
	m = send(m, tasksLoadedMsg{seq: m.loadSeq, myTasks: tasks("Work", 2)})
	if len(m.myTasks) != 2 || m.loadingTasks {
		t.Fatal("current response should be applied")
	}
}

func TestPartialFailureKeepsFailedProfileData(t *testing.T) {
	ok := &fakeBackend{name: "A", tasks: tasks("A", 2)}
	bad := &fakeBackend{name: "B", err: errors.New("boom")}
	m := newTestModel(ok, bad)
	m.myTasks = append(tasks("A", 1), tasks("B", 1)...)

	cmd := m.startLoad()
	m = send(m, cmd())

	if len(m.myTasks) != 3 { // 2 fresh from A + 1 kept from B
		t.Errorf("my tasks = %d, want 3", len(m.myTasks))
	}
	if !strings.Contains(m.statusMsg, "B: boom") {
		t.Errorf("status should report the failing profile, got %q", m.statusMsg)
	}
}

func TestAllProfilesFailingKeepsData(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A", err: errors.New("down")})
	m.myTasks = tasks("A", 4)
	cmd := m.startLoad()
	m = send(m, cmd())
	if len(m.myTasks) != 4 || m.err == nil {
		t.Errorf("data should be kept and error set, my=%d err=%v", len(m.myTasks), m.err)
	}
}

func TestDateFilterWidensBackendScope(t *testing.T) {
	filters := make(chan config.Filter, 3)
	m := newTestModel(&fakeBackend{name: "A", filters: filters})

	// Cycle to "last quarter", which reaches past the default week.
	var cmd tea.Cmd
	for range 8 {
		m, cmd = press(t, m, "f")
	}
	if v, _ := m.activeCycleValue("date"); v != "last quarter" {
		t.Fatalf("date filter = %q", v)
	}
	if m.scopeDays <= defaultScopeDays || cmd == nil {
		t.Fatalf("scope should widen and reload, scope=%d", m.scopeDays)
	}
	cmd()
	if got := (<-filters).UpdatedSince; got != fmt.Sprintf("last_%d_days", m.scopeDays) {
		t.Errorf("backend queried with %q", got)
	}

	m, cmd = press(t, m, "esc")
	if m.scopeDays != defaultScopeDays || cmd == nil {
		t.Errorf("clearing the filter should restore the default scope, got %d", m.scopeDays)
	}
}

func TestScrollingKeepsCursorAndHeaderVisible(t *testing.T) {
	m := newTestModel()
	m.myTasks = tasks("A", 100)
	m.height = 20

	m, _ = press(t, m, "G")
	if m.cursor != 99 {
		t.Fatalf("cursor = %d", m.cursor)
	}
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) > m.height {
		t.Errorf("view has %d lines for height %d", len(lines), m.height)
	}
	if !strings.Contains(lines[0], "canopy") {
		t.Error("title should stay visible")
	}
	if !strings.Contains(out, "Task 99") || strings.Contains(out, "Task 0 ") {
		t.Error("list should be scrolled to the cursor")
	}

	m, _ = press(t, m, "g", "g")
	if m.offset != 0 {
		t.Errorf("gg should scroll to top, offset = %d", m.offset)
	}
}

func TestFilterBackspaceKeepsUTF8(t *testing.T) {
	m := newTestModel()
	m, _ = press(t, m, "/", "b", "l", "å")
	m, _ = press(t, m, "backspace")
	if m.filterQuery != "bl" {
		t.Errorf("filterQuery = %q, want %q", m.filterQuery, "bl")
	}
}

func TestCreateFormUsesParentProfile(t *testing.T) {
	a := &fakeBackend{name: "A"}
	b := &fakeBackend{name: "B"}
	m := newTestModel(a, b)
	m.cfg.Profiles = []config.Profile{{Name: "A", Team: []string{"ann"}}, {Name: "B", Team: []string{"bob"}}}
	m.navStack = []model.Task{{ID: "7", Profile: "B", Type: model.TypeFeature}}

	m.openCreateForm()
	if !m.showForm || m.form.creator != backend.TaskCreator(b) {
		t.Fatal("form should create in the parent's profile")
	}
	if m.form.values[formFieldAssignee] != "bob" || formTypes[m.form.typeIdx] != model.TypeUserStory {
		t.Errorf("defaults: assignee=%q type=%s", m.form.values[formFieldAssignee], formTypes[m.form.typeIdx])
	}

	m, _ = press(t, m, "S", "ø", "backspace")
	if m.form.values[formFieldTitle] != "S" {
		t.Errorf("title = %q", m.form.values[formFieldTitle])
	}
	m.form.values[formFieldStartDate] = "2026-13-01"
	if err := m.form.validate(); err != "start date must be YYYY-MM-DD" {
		t.Errorf("validate = %q", err)
	}
}

func TestTaskLoadDoesNotHideViewError(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"})
	m.activeTab = tabViews
	m.viewIdx = 0
	m.viewSeq = 1
	m = send(m, viewLoadedMsg{seq: 1, failures: []profileFailure{{"A", errors.New("view down")}}})
	cmd := m.startLoad()
	m = send(m, cmd())
	if m.viewErr == nil || !strings.Contains(m.statusMsg, "view down") {
		t.Errorf("view failure hidden: viewErr=%v status=%q", m.viewErr, m.statusMsg)
	}
	if !strings.Contains(m.View(), "view down") {
		t.Error("empty failed view should show its error")
	}
}

func TestChildTasksPreferViewCopy(t *testing.T) {
	m := newTestModel()
	m.myTasks = []model.Task{{ID: "2", ParentID: "1", State: model.StateInProgress}}
	m.viewTasks = []model.Task{{ID: "2", ParentID: "1", State: model.StateDone}}
	if got := m.childTasks("1"); len(got) != 1 || got[0].State != model.StateDone {
		t.Errorf("children = %+v", got)
	}
}

func TestBucketRanges(t *testing.T) {
	now := time.Date(2026, 10, 2, 15, 0, 0, 0, time.UTC) // a Friday

	if !dateInBucket(time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC), "this week", now) {
		t.Error("Monday should be in this week")
	}
	if dateInBucket(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), "this month", now) == false {
		t.Error("later this month should be in this month")
	}
	if dateInBucket(time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), "this month", now) {
		t.Error("next month must not be in this month")
	}
	if dateInBucket(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), "today", now) {
		t.Error("tomorrow must not be today")
	}
	if !dateInBucket(time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC), "last 6 months", now) {
		t.Error("six-month buckets should start at midnight")
	}

	cases := map[string]int{"today": 0, "this week": 4, "last week": 11, "this month": 1, "last quarter": 93}
	for label, want := range cases {
		if got := bucketDays(label, now); got != want {
			t.Errorf("bucketDays(%q) = %d, want %d", label, got, want)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.0", "0.2.1", true},
		{"v0.2.1", "0.2.1", false},
		{"v0.2.0", "0.2.1", false},
		{"v0.10.0", "0.9.9", true},
		{"v1.0.0-rc1", "0.2.1", false},
		{"v0.3.0", "dev-0123456789ab", false},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v", c.latest, c.current, got)
		}
	}
}

func TestDueIndicatorIgnoresFinishedTasks(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3)
	done := dueIndicator(model.Task{TargetDate: past, State: model.StateDone})
	open := dueIndicator(model.Task{TargetDate: past, State: model.StateInProgress})
	if strings.Contains(done, "!") || strings.Contains(done, "●") {
		t.Errorf("done task flagged as due: %q", done)
	}
	if !strings.Contains(open, "!") {
		t.Errorf("open overdue task not flagged: %q", open)
	}
}

func TestViewRefreshKeepsFailedProfileResults(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"}, &fakeBackend{name: "B"})
	m.activeTab = tabViews
	m.viewIdx = 0
	m.viewTasks = append(tasks("A", 1), tasks("B", 2)...)
	m.viewSeq = 1

	m = send(m, viewLoadedMsg{seq: 1, tasks: tasks("A", 3), failures: []profileFailure{{"B", errors.New("down")}}})
	if len(m.viewTasks) != 5 {
		t.Errorf("view tasks = %d, want 3 fresh from A + 2 kept from B", len(m.viewTasks))
	}
}

func TestIterationResultForClosedFormIsIgnored(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"})
	m.openCreateForm()
	stale := m.formSeq
	m, _ = press(t, m, "esc")
	m.openCreateForm()

	m = send(m, iterationResolvedMsg{formSeq: stale, path: `A\Old`})
	if got := m.form.values[formFieldIteration]; got != "" {
		t.Errorf("stale lookup filled sprint with %q", got)
	}
	m = send(m, iterationResolvedMsg{formSeq: m.formSeq, path: `A\Current`})
	if got := m.form.values[formFieldIteration]; got != `A\Current` {
		t.Errorf("sprint = %q", got)
	}
}

func TestTruncatedResultsAreKeptAndReported(t *testing.T) {
	b := &fakeBackend{name: "A", tasks: tasks("A", 2), err: fmt.Errorf("%w at 2 items", backend.ErrTruncated)}
	m := newTestModel(b)
	cmd := m.startLoad()
	m = send(m, cmd())
	if len(m.myTasks) != 2 || m.err != nil {
		t.Errorf("truncated results should be applied, my=%d err=%v", len(m.myTasks), m.err)
	}
	if !strings.Contains(m.statusMsg, "capped") {
		t.Errorf("status should mention the cap: %q", m.statusMsg)
	}
}

func TestPartialFailureDoesNotReplaceEmptyListMessage(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"}, &fakeBackend{name: "Stub", err: errors.New("not yet implemented")})
	cmd := m.startLoad()
	m = send(m, cmd())
	if out := m.View(); !strings.Contains(out, "No tasks found") || !strings.Contains(m.statusMsg, "Stub") {
		t.Errorf("status = %q, view:\n%s", m.statusMsg, out)
	}
}

func TestClosingViewRestoresTaskStatus(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A", tasks: tasks("A", 1)})
	cmd := m.startLoad()
	m = send(m, cmd())
	taskStatus := m.statusMsg

	m.activeTab = tabViews
	m.viewIdx = 0
	m.viewSeq = 1
	m = send(m, viewLoadedMsg{seq: 1, failures: []profileFailure{{"A", errors.New("no current sprint")}}})
	m, _ = press(t, m, "esc")
	if m.statusMsg != taskStatus {
		t.Errorf("status after leaving view = %q, want %q", m.statusMsg, taskStatus)
	}
}

func TestInfoBarShowsLoadedScope(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"})
	m.scopeDays = 93 // requested, not loaded yet
	if strings.Contains(m.infoBarText(), "93 days") {
		t.Error("info bar should show the loaded window until the load completes")
	}
	m = send(m, tasksLoadedMsg{seq: m.loadSeq, days: 93})
	if !strings.Contains(m.infoBarText(), "last 93 days") {
		t.Errorf("info bar = %q", m.infoBarText())
	}
}

func TestEmptyViewIgnoresTaskLoading(t *testing.T) {
	m := newTestModel(&fakeBackend{name: "A"})
	m.activeTab = tabViews
	m.viewIdx = 0
	m.loadingTasks = true
	if out := m.View(); strings.Contains(out, "Loading…") || !strings.Contains(out, "No tasks found") {
		t.Errorf("empty view while tabs load:\n%s", out)
	}
}
