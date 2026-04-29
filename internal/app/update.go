package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/model"
)

func (m Model) Init() tea.Cmd {
	if len(m.backends) == 0 {
		return nil
	}
	m.loading = true
	return tea.Batch(
		m.loadAllTasks(),
		tickCmd(time.Duration(m.cfg.RefreshSecs)*time.Second),
		checkLatestVersion(m.version),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Overlays intercept keys first.
	if m.showHelp {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleHelpKey(km)
		}
	}
	if m.showSplash {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleSplashKey(km)
		}
	}
	if m.showDetail {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleDetailKey(km)
		}
	}
	if m.showForm {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleFormKey(km)
		}
	}

	// In text-filter mode, handle input first.
	if m.filtering {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handleFilterKey(km)
		}
	}

	switch msg := msg.(type) {
	case tasksLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.err = nil
			m.myTasks = msg.myTasks
			m.teamTasks = msg.teamTasks
			m.doneTasks = msg.doneTasks
			m.tasksLoadedAt = time.Now()
			m.statusMsg = fmt.Sprintf("%d my · %d team · %d done",
				len(m.myTasks), len(m.teamTasks), len(m.doneTasks))
			m.saveCachedTasks()
		}
		return m, nil

	case taskCreatedMsg:
		m.formSubmitting = false
		if msg.err != nil {
			m.formErr = msg.err.Error()
			return m, nil
		}
		m.showForm = false
		m.statusMsg = fmt.Sprintf("Created %s #%s: %s", msg.task.Type, msg.task.ID, msg.task.Title)
		m.loading = true
		return m, m.loadAllTasks()

	case iterationResolvedMsg:
		if msg.err == nil {
			m.formIteration = msg.path
		}
		return m, nil

	case tickMsg:
		if len(m.backends) > 0 {
			return m, tea.Batch(
				m.loadAllTasks(),
				tickCmd(time.Duration(m.cfg.RefreshSecs)*time.Second),
			)
		}
		return m, nil

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

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "q", "ctrl+c":
		return m, tea.Quit

	// Overlays
	case "?":
		m.showHelp = true
		return m, nil
	case "!":
		m.showSplash = true
		return m, nil

	// Tab navigation
	case "h":
		if m.activeTab > 0 {
			m.activeTab--
			m.cursor = 0
			m.navStack = nil
			m.clearCycleFilter()
			m.filterQuery = ""
		}
	case "l":
		if m.activeTab < tabViews {
			m.activeTab++
			m.cursor = 0
			m.navStack = nil
			m.clearCycleFilter()
			m.filterQuery = ""
		}
	case "1":
		m.activeTab = tabMyTasks
		m.cursor = 0
		m.navStack = nil
		m.clearCycleFilter()
		m.filterQuery = ""
	case "2":
		m.activeTab = tabTeam
		m.cursor = 0
		m.navStack = nil
		m.clearCycleFilter()
		m.filterQuery = ""
	case "3":
		m.activeTab = tabDone
		m.cursor = 0
		m.navStack = nil
		m.clearCycleFilter()
		m.filterQuery = ""
	case "4":
		m.activeTab = tabViews
		m.cursor = 0
		m.navStack = nil
		m.clearCycleFilter()
		m.filterQuery = ""

	// List navigation
	case "j", "down":
		m.cursor++
		m.clampCursor()
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "G":
		m.cursor = m.listLen() - 1
		if m.cursor < 0 {
			m.cursor = 0
		}

	// Cycle filters
	case "f":
		if m.activeTab != tabViews {
			m.doCycleFilter("date")
		}
	case "F":
		if m.activeTab != tabViews {
			m.doCycleDateField()
		}
	case "d":
		if m.activeTab != tabViews {
			m.doCycleFilter("assignee")
		}
	case "s":
		if m.activeTab != tabViews {
			m.doCycleFilter("type")
		}
	case "t":
		if m.activeTab != tabViews {
			m.doCycleFilter("tag")
		}

	// Text search
	case "/":
		if m.activeTab != tabViews {
			m.filtering = true
			m.filterQuery = ""
		}

	// Clear filters / navigate back
	case "esc":
		if m.cycleField != "" || m.filterQuery != "" {
			m.clearCycleFilter()
			m.filterQuery = ""
			m.cursor = 0
		} else if len(m.navStack) > 0 {
			m.navStack = m.navStack[:len(m.navStack)-1]
			m.cursor = 0
		}
	case "backspace":
		if len(m.navStack) > 0 {
			m.navStack = m.navStack[:len(m.navStack)-1]
			m.cursor = 0
		}

	// Actions
	case "c":
		if m.activeTab != tabViews && m.canCreate() {
			m.showForm = true
			m.formField = formFieldTitle
			m.formTitle = ""
			m.formDesc = ""
			m.formTags = ""
			m.formStartDate = ""
			m.formTargetDate = ""
			m.formAcceptCriteria = ""
			m.formErr = ""
			m.formSubmitting = false
			m.formType = m.defaultFormTypeIndex()
			m.formAssignee = m.defaultAssignee()
			m.formIteration = ""
			return m, m.resolveIteration()
		}

	case "r":
		if len(m.backends) > 0 {
			m.loading = true
			m.statusMsg = "Refreshing..."
			return m, m.loadAllTasks()
		}
	case "enter":
		if m.activeTab == tabViews && m.cursor < len(m.cfg.Views) {
			v := m.cfg.Views[m.cursor]
			m.loading = true
			m.statusMsg = fmt.Sprintf("Loading %s...", v.Name)
			return m, m.loadViewTasks(v.Filters)
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
		if t, ok := m.taskAtCursor(); ok && t.URL != "" {
			copyToClipboard(t.URL)
			m.statusMsg = "copied URL to clipboard"
		}
	case "o":
		m.openInBrowser()
	case "[":
		if len(m.navStack) > 0 {
			m.navigateSibling(-1)
		}
	case "]":
		if len(m.navStack) > 0 {
			m.navigateSibling(1)
		}
	}

	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "?", "esc", "q":
		m.showHelp = false
	}
	return m, nil
}

func (m Model) handleSplashKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "!", "esc", "q":
		m.showSplash = false
	default:
		m.showSplash = false
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			openURL(m.detailTask.URL)
		}
	case " ":
		if m.detailTask.URL != "" {
			copyToClipboard(m.detailTask.URL)
			m.statusMsg = "copied URL to clipboard"
			m.showDetail = false
		}
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterQuery = ""
		m.cursor = 0
		return m, nil
	case "enter":
		m.filtering = false
		m.cursor = 0
		return m, nil
	case "backspace":
		if len(m.filterQuery) > 0 {
			m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
			m.cursor = 0
		}
		return m, nil
	default:
		r := msg.Runes
		if len(r) > 0 {
			m.filterQuery += string(r)
			m.cursor = 0
		}
		return m, nil
	}
}

func (m *Model) clampCursor() {
	max := m.listLen() - 1
	if max < 0 {
		max = 0
	}
	if m.cursor > max {
		m.cursor = max
	}
}

func (m Model) listLen() int {
	if m.activeTab == tabViews {
		return len(m.cfg.Views)
	}
	return len(m.currentTasks())
}

func (m Model) taskAtCursor() (model.Task, bool) {
	if m.activeTab == tabViews {
		return model.Task{}, false
	}
	tasks := m.currentTasks()
	if m.cursor >= len(tasks) {
		return model.Task{}, false
	}
	return tasks[m.cursor], true
}

func (m Model) openInBrowser() {
	if t, ok := m.taskAtCursor(); ok && t.URL != "" {
		openURL(t.URL)
	}
}

// currentTasks returns the task list for the active tab, filtered by the
// current navigation stack (children of the deepest parent) and any active
// text/cycle filters.
func (m Model) currentTasks() []model.Task {
	if len(m.navStack) > 0 {
		parent := m.navStack[len(m.navStack)-1]
		return m.filteredTasks(m.childTasks(parent.ID))
	}
	switch m.activeTab {
	case tabMyTasks:
		return m.filteredTasks(m.myTasks)
	case tabTeam:
		return m.filteredTasks(m.teamTasks)
	case tabDone:
		return m.filteredTasks(m.doneTasks)
	}
	return nil
}

// childTasks returns all loaded tasks whose ParentID matches the given ID,
// deduplicated across all three task lists.
func (m Model) childTasks(parentID string) []model.Task {
	seen := make(map[string]bool)
	var children []model.Task
	for _, list := range [][]model.Task{m.myTasks, m.teamTasks, m.doneTasks} {
		for _, t := range list {
			if t.ParentID == parentID && !seen[t.ID] {
				seen[t.ID] = true
				children = append(children, t)
			}
		}
	}
	return children
}

// siblingTasks returns the task list at the same level as the top of the
// navStack — i.e. the list the current parent was selected from.
func (m Model) siblingTasks() []model.Task {
	if len(m.navStack) > 1 {
		grandparent := m.navStack[len(m.navStack)-2]
		return m.filteredTasks(m.childTasks(grandparent.ID))
	}
	switch m.activeTab {
	case tabMyTasks:
		return m.filteredTasks(m.myTasks)
	case tabTeam:
		return m.filteredTasks(m.teamTasks)
	case tabDone:
		return m.filteredTasks(m.doneTasks)
	}
	return nil
}

// navigateSibling replaces the top of the navStack with the previous or next
// sibling task (delta = -1 or +1).
func (m *Model) navigateSibling(delta int) {
	if len(m.navStack) == 0 {
		return
	}
	siblings := m.siblingTasks()
	current := m.navStack[len(m.navStack)-1]

	idx := -1
	for i, t := range siblings {
		if t.ID == current.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}

	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(siblings) {
		return
	}

	m.navStack[len(m.navStack)-1] = siblings[newIdx]
	m.cursor = 0
}
