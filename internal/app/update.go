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
	switch msg := msg.(type) {

	case tasksLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.err = nil
			m.statusMsg = ""
			m.myTasks = msg.myTasks
			m.teamTasks = msg.teamTasks
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
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	// Tab navigation
	case "h":
		if m.activeTab > 0 {
			m.activeTab--
			m.cursor = 0
		}
	case "l":
		if m.activeTab < tabViews {
			m.activeTab++
			m.cursor = 0
		}
	case "1":
		m.activeTab = tabMyTasks
		m.cursor = 0
	case "2":
		m.activeTab = tabTeam
		m.cursor = 0
	case "3":
		m.activeTab = tabViews
		m.cursor = 0

	// List navigation
	case "j":
		m.cursor++
		m.clampCursor()
	case "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = m.listLen() - 1
		if m.cursor < 0 {
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
		return len(m.myTasks)
	case tabTeam:
		return len(m.teamTasks)
	case tabViews:
		return len(m.cfg.Views)
	}
	return 0
}

func (m Model) openInBrowser() {
	var tasks []model.Task
	switch m.activeTab {
	case tabMyTasks:
		tasks = m.myTasks
	case tabTeam:
		tasks = m.teamTasks
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
