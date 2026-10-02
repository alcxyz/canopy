package app

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/alcxyz/canopy/internal/platform"
)

func (m Model) Init() tea.Cmd {
	if len(m.backends) == 0 {
		return nil
	}
	// Init cannot keep the cancel function (it has a value receiver), so the
	// first load is only bounded by its timeout.
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	return tea.Batch(
		loadTasks(ctx, cancel, m.backends, m.loadSeq, m.scopeDays),
		tickCmd(m.refreshInterval()),
		checkLatestVersion(m.version),
	)
}

func (m Model) refreshInterval() time.Duration {
	return time.Duration(m.cfg.RefreshSecs) * time.Second
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := m.update(msg)
	m.clampCursor()
	m.scrollToCursor()
	return m, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Overlays and text input intercept keys first.
		switch {
		case m.showHelp:
			return m.handleHelpKey(km)
		case m.showSplash:
			m.showSplash = false
			return m, nil
		case m.showDetail:
			return m.handleDetailKey(km)
		case m.showForm:
			return m.handleFormKey(km)
		case m.filtering:
			return m.handleFilterKey(km)
		}
		return m.handleKey(km)
	}

	switch msg := msg.(type) {
	case tasksLoadedMsg:
		return m.applyTasks(msg), nil

	case viewLoadedMsg:
		return m.applyView(msg), nil

	case taskCreatedMsg:
		m.form.submitting = false
		if msg.err != nil {
			m.form.err = msg.err.Error()
			return m, nil
		}
		m.showForm = false
		m.statusMsg = fmt.Sprintf("Created %s #%s: %s", msg.task.Type, msg.task.ID, msg.task.Title)
		return m, m.refresh()

	case iterationResolvedMsg:
		switch {
		case msg.formSeq != m.formSeq || !m.showForm:
			// The lookup belongs to a form that has since closed.
		case msg.err != nil:
			log.Printf("resolving current iteration: %v", msg.err)
		case m.form.values[formFieldIteration] == "":
			m.form.values[formFieldIteration] = msg.path
		}
		return m, nil

	case openResultMsg:
		if msg.err != nil {
			m.statusMsg = "open failed: " + msg.err.Error()
		}
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{tickCmd(m.refreshInterval())}
		if !m.loadingTasks {
			cmds = append(cmds, m.startLoad())
		}
		if m.viewOpen() && !m.loadingView {
			cmds = append(cmds, m.startViewLoad())
		}
		return m, tea.Batch(cmds...)

	case versionCheckMsg:
		m.latestVersion = msg.latest
		return m, nil

	case ggTimeoutMsg:
		m.prevKey = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil
	}

	return m, nil
}

// applyTasks stores the result of a task load. Profiles that failed keep
// their previously loaded tasks. While a view is open the status bar belongs
// to the view, so only the task data and error are updated.
func (m Model) applyTasks(msg tasksLoadedMsg) Model {
	if msg.seq != m.loadSeq {
		return m // superseded by a newer request
	}
	m.loadingTasks = false
	logFailures(msg.failures)

	var status string
	if m.allFailed(msg.failures) {
		m.err = msg.failures[0].err
		status = "Error: " + failureSummary(msg.failures)
	} else {
		failed := failedProfiles(msg.failures)
		m.myTasks = keepFailedProfiles(m.myTasks, msg.myTasks, failed)
		m.teamTasks = keepFailedProfiles(m.teamTasks, msg.teamTasks, failed)
		m.doneTasks = keepFailedProfiles(m.doneTasks, msg.doneTasks, failed)
		m.tasksLoadedAt = time.Now()
		m.loadedDays = msg.days
		m.saveCachedTasks()
		m.err = nil // partial failures are reported in the status bar
		status = fmt.Sprintf("%d my · %d team · %d done",
			len(m.myTasks), len(m.teamTasks), len(m.doneTasks)) + loadNotes(msg.truncated, msg.failures)
	}
	m.tasksStatus = status
	if !m.viewOpen() {
		m.statusMsg = status
	}
	return m
}

// applyView stores the result of a view load.
func (m Model) applyView(msg viewLoadedMsg) Model {
	if msg.seq != m.viewSeq || !m.viewOpen() {
		return m
	}
	m.loadingView = false
	logFailures(msg.failures)

	if m.allFailed(msg.failures) {
		m.viewErr = msg.failures[0].err
		m.statusMsg = "Error: " + failureSummary(msg.failures)
		return m
	}
	m.viewTasks = keepFailedProfiles(m.viewTasks, msg.tasks, failedProfiles(msg.failures))
	m.viewErr = nil
	m.statusMsg = fmt.Sprintf("%s: %d tasks", m.cfg.Views[m.viewIdx].Name, len(m.viewTasks)) +
		loadNotes(msg.truncated, msg.failures)
	return m
}

func (m Model) allFailed(failures []profileFailure) bool {
	return len(failures) > 0 && len(failures) == len(m.backends)
}

func failedProfiles(failures []profileFailure) map[string]bool {
	failed := make(map[string]bool, len(failures))
	for _, f := range failures {
		failed[f.profile] = true
	}
	return failed
}

// loadNotes describes capped results and failed profiles for the status bar.
func loadNotes(truncated bool, failures []profileFailure) string {
	var s string
	if truncated {
		s += truncatedNote
	}
	if len(failures) > 0 {
		s += " · failed " + failureSummary(failures)
	}
	return s
}

// truncatedNote is appended to the status when a backend capped its results.
const truncatedNote = " · some results capped (newest kept)"

// keepFailedProfiles returns fresh plus the previous tasks of profiles whose
// refresh failed, so a failing profile keeps showing its last known data.
func keepFailedProfiles(prev, fresh []model.Task, failed map[string]bool) []model.Task {
	if len(failed) == 0 {
		return fresh
	}
	for _, t := range prev {
		if failed[t.Profile] {
			fresh = append(fresh, t)
		}
	}
	return fresh
}

func logFailures(failures []profileFailure) {
	for _, f := range failures {
		log.Printf("profile %q: %v", f.profile, f.err)
	}
}

func failureSummary(failures []profileFailure) string {
	s := fmt.Sprintf("%s: %v", failures[0].profile, failures[0].err)
	if len(failures) > 1 {
		s += fmt.Sprintf(" (+%d more, see log)", len(failures)-1)
	}
	return s
}

// refresh reloads the task tabs and, when one is open, the current view.
func (m *Model) refresh() tea.Cmd {
	if len(m.backends) == 0 {
		return nil
	}
	cmds := []tea.Cmd{m.startLoad()}
	if m.viewOpen() {
		cmds = append(cmds, m.startViewLoad())
	}
	return tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// gg — go to first item (vim-style double-g with timeout)
	if key == "g" {
		if m.prevKey == "g" && time.Since(m.prevKeyAt) < m.ggTimeout {
			m.cursor = 0
			m.prevKey = ""
			return m, nil
		}
		m.prevKey = "g"
		m.prevKeyAt = time.Now()
		return m, tea.Tick(m.ggTimeout, func(time.Time) tea.Msg { return ggTimeoutMsg{} })
	}
	m.prevKey = ""

	switch key {
	case "q":
		return m, tea.Quit

	// Overlays
	case "?":
		m.showHelp = true
	case "!":
		m.showSplash = true

	// Tab navigation
	case "h":
		if m.activeTab > 0 {
			return m, m.switchTab(m.activeTab - 1)
		}
	case "l":
		if m.activeTab < tabViews {
			return m, m.switchTab(m.activeTab + 1)
		}
	case "1", "2", "3", "4":
		return m, m.switchTab(tab(key[0] - '1'))

	// List navigation
	case "j", "down":
		m.cursor++
	case "k", "up":
		m.cursor--
	case "G":
		m.cursor = m.listLen() - 1
	case "ctrl+d", "pgdown":
		m.cursor += m.pageStep(key)
	case "ctrl+u", "pgup":
		m.cursor -= m.pageStep(key)

	// Cycle filters
	case "f":
		if m.showsTasks() {
			m.doCycleFilter("date")
			return m, m.syncScope()
		}
	case "F":
		if m.showsTasks() {
			m.doCycleDateField()
		}
	case "d":
		if m.showsTasks() {
			m.doCycleFilter("assignee")
			return m, m.syncScope()
		}
	case "s":
		if m.showsTasks() {
			m.doCycleFilter("type")
			return m, m.syncScope()
		}
	case "t":
		if m.showsTasks() {
			m.doCycleFilter("tag")
			return m, m.syncScope()
		}

	// Text search
	case "/":
		if m.showsTasks() {
			m.filtering = true
			m.filterQuery = ""
		}

	// Clear filters / navigate back
	case "esc":
		switch {
		case m.hasFilters():
			m.clearFilters()
			return m, m.syncScope()
		case len(m.navStack) > 0:
			m.popNav()
		case m.viewOpen():
			m.closeView()
		}
	case "backspace":
		switch {
		case len(m.navStack) > 0:
			m.popNav()
		case m.viewOpen():
			m.closeView()
		}

	// Actions
	case "c":
		if m.activeTab != tabViews {
			return m, m.openCreateForm()
		}
	case "r":
		if len(m.backends) > 0 {
			m.statusMsg = "Refreshing..."
			return m, m.refresh()
		}
	case "enter":
		if m.activeTab == tabViews && !m.viewOpen() {
			return m, m.openView(m.cursor)
		}
		if t, ok := m.taskAtCursor(); ok {
			m.navStack = append(m.navStack, t)
			m.cursor = 0
		}
	case "i":
		if t, ok := m.taskAtCursor(); ok {
			m.showDetail = true
			m.detailTask = t
		}
	case " ":
		if t, ok := m.taskAtCursor(); ok {
			m.copyURL(t)
		}
	case "o":
		if t, ok := m.taskAtCursor(); ok && t.URL != "" {
			return m, openURL(t.URL)
		}
	case "[":
		m.navigateSibling(-1)
	case "]":
		m.navigateSibling(1)
	}

	return m, nil
}

// pageStep returns the cursor jump for paging keys: half a page for
// ctrl+d/ctrl+u, a full page otherwise.
func (m Model) pageStep(key string) int {
	rows := m.visibleRows()
	if key == "ctrl+d" || key == "ctrl+u" {
		return max(1, rows/2)
	}
	return rows
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q":
		m.showHelp = false
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showDetail = false
	case "enter":
		m.showDetail = false
		m.navStack = append(m.navStack, m.detailTask)
		m.cursor = 0
	case "[":
		if m.cursor > 0 {
			m.cursor--
			if t, ok := m.taskAtCursor(); ok {
				m.detailTask = t
			}
		}
	case "]":
		tasks := m.currentTasks()
		if m.cursor < len(tasks)-1 {
			m.cursor++
			m.detailTask = tasks[m.cursor]
		}
	case "o":
		if m.detailTask.URL != "" {
			return m, openURL(m.detailTask.URL)
		}
	case " ":
		m.copyURL(m.detailTask)
		m.showDetail = false
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterQuery = ""
	case "enter":
		m.filtering = false
	case "backspace":
		m.filterQuery = dropLastRune(m.filterQuery)
	default:
		m.filterQuery += string(msg.Runes)
	}
	m.cursor = 0
	return m, nil
}

func (m *Model) copyURL(t model.Task) {
	if t.URL == "" {
		return
	}
	if err := platform.CopyToClipboard(t.URL); err != nil {
		m.statusMsg = "copy failed: " + err.Error()
		return
	}
	m.statusMsg = "copied URL to clipboard"
}

// ── Cursor and scrolling ────────────────────────────────────────────────

func (m *Model) clampCursor() {
	m.cursor = max(0, min(m.cursor, m.listLen()-1))
}

// scrollToCursor adjusts the scroll offset so the cursor row is visible.
func (m *Model) scrollToCursor() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, m.listLen()-rows))
}

// ── Tabs, views and navigation ──────────────────────────────────────────

func (m *Model) switchTab(next tab) tea.Cmd {
	m.closeView()
	m.activeTab = next
	m.cursor = 0
	m.navStack = nil
	m.clearFilters()
	m.saveState()
	return m.syncScope()
}

// viewOpen reports whether a configured view's tasks are being shown.
func (m Model) viewOpen() bool {
	return m.activeTab == tabViews && m.viewIdx >= 0 && m.viewIdx < len(m.cfg.Views)
}

// showsTasks reports whether the active tab is showing a task list.
func (m Model) showsTasks() bool {
	return m.activeTab != tabViews || m.viewOpen()
}

func (m *Model) openView(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.cfg.Views) {
		return nil
	}
	m.viewIdx = idx
	m.viewTasks = nil
	m.cursor = 0
	if len(m.backends) == 0 {
		return nil
	}
	m.statusMsg = fmt.Sprintf("Loading %s...", m.cfg.Views[idx].Name)
	return m.startViewLoad()
}

// closeView returns to the view list, dropping any pending view load.
func (m *Model) closeView() {
	if m.viewIdx < 0 {
		return
	}
	m.navStack = nil
	m.clearFilters()
	m.cursor = m.viewIdx
	m.viewIdx = -1
	m.viewTasks = nil
	m.viewErr = nil
	m.viewSeq++
	m.loadingView = false
	if m.cancelView != nil {
		m.cancelView()
		m.cancelView = nil
	}
	m.statusMsg = m.tasksStatus
}

func (m *Model) popNav() {
	m.navStack = m.navStack[:len(m.navStack)-1]
	m.cursor = 0
}

func (m Model) listLen() int {
	if !m.showsTasks() {
		return len(m.cfg.Views)
	}
	return len(m.currentTasks())
}

func (m Model) taskAtCursor() (model.Task, bool) {
	if !m.showsTasks() {
		return model.Task{}, false
	}
	tasks := m.currentTasks()
	if m.cursor < 0 || m.cursor >= len(tasks) {
		return model.Task{}, false
	}
	return tasks[m.cursor], true
}

// tabTasks returns the unfiltered top-level task list for the active tab.
func (m Model) tabTasks() []model.Task {
	switch m.activeTab {
	case tabMyTasks:
		return m.myTasks
	case tabTeam:
		return m.teamTasks
	case tabDone:
		return m.doneTasks
	case tabViews:
		return m.viewTasks
	}
	return nil
}

// levelTasks returns the unfiltered tasks at a navigation depth: the tab's
// list at depth 0, otherwise the children of navStack[depth-1].
func (m Model) levelTasks(depth int) []model.Task {
	if depth <= 0 {
		return m.tabTasks()
	}
	return m.childTasks(m.navStack[depth-1].ID)
}

// currentTasks returns the filtered tasks at the current navigation level.
func (m Model) currentTasks() []model.Task {
	return m.filteredTasks(m.levelTasks(len(m.navStack)))
}

// childTasks returns all loaded tasks whose ParentID matches the given ID,
// deduplicated across the task lists. View results come first because an
// open view is usually fresher than the task tabs.
func (m Model) childTasks(parentID string) []model.Task {
	seen := make(map[string]bool)
	var children []model.Task
	for _, list := range [][]model.Task{m.viewTasks, m.myTasks, m.teamTasks, m.doneTasks} {
		for _, t := range list {
			if t.ParentID == parentID && !seen[t.ID] {
				seen[t.ID] = true
				children = append(children, t)
			}
		}
	}
	return children
}

// navigateSibling replaces the top of the navStack with the previous or next
// sibling task (delta = -1 or +1).
func (m *Model) navigateSibling(delta int) {
	depth := len(m.navStack)
	if depth == 0 {
		return
	}
	siblings := m.filteredTasks(m.levelTasks(depth - 1))
	current := m.navStack[depth-1]

	for i, t := range siblings {
		if t.ID != current.ID {
			continue
		}
		if next := i + delta; next >= 0 && next < len(siblings) {
			m.navStack[depth-1] = siblings[next]
			m.cursor = 0
		}
		return
	}
}
