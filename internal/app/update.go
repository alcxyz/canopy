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
			m.statusMsg = fmt.Sprintf("%d my · %d team · %d done",
				len(m.myTasks), len(m.teamTasks), len(m.doneTasks))
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
			m.clearCycleFilter()
			m.filterQuery = ""
		}
	case "l":
		if m.activeTab < tabViews {
			m.activeTab++
			m.cursor = 0
			m.clearCycleFilter()
			m.filterQuery = ""
		}
	case "1":
		m.activeTab = tabMyTasks
		m.cursor = 0
		m.clearCycleFilter()
		m.filterQuery = ""
	case "2":
		m.activeTab = tabTeam
		m.cursor = 0
		m.clearCycleFilter()
		m.filterQuery = ""
	case "3":
		m.activeTab = tabDone
		m.cursor = 0
		m.clearCycleFilter()
		m.filterQuery = ""
	case "4":
		m.activeTab = tabViews
		m.cursor = 0
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
	case "d":
		if m.activeTab != tabViews {
			m.doCycleFilter("assignee")
		}
	case "s":
		if m.activeTab != tabViews {
			m.doCycleFilter("type")
		}

	// Text search
	case "/":
		if m.activeTab != tabViews {
			m.filtering = true
			m.filterQuery = ""
		}

	// Clear filters
	case "esc":
		if m.cycleField != "" || m.filterQuery != "" {
			m.clearCycleFilter()
			m.filterQuery = ""
			m.cursor = 0
		}

	// Actions
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
	case "o":
		m.openInBrowser()
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
	switch m.activeTab {
	case tabMyTasks:
		return len(m.filteredTasks(m.myTasks))
	case tabTeam:
		return len(m.filteredTasks(m.teamTasks))
	case tabDone:
		return len(m.filteredTasks(m.doneTasks))
	case tabViews:
		return len(m.cfg.Views)
	}
	return 0
}

func (m Model) openInBrowser() {
	var tasks []model.Task
	switch m.activeTab {
	case tabMyTasks:
		tasks = m.filteredTasks(m.myTasks)
	case tabTeam:
		tasks = m.filteredTasks(m.teamTasks)
	case tabDone:
		tasks = m.filteredTasks(m.doneTasks)
	default:
		return
	}
	if m.cursor >= len(tasks) {
		return
	}
	url := tasks[m.cursor].URL
	if url == "" {
		return
	}
	openURL(url)
}
