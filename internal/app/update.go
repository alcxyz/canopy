package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
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
		case "j":
			m.cursor++
			m.clampCursor()
		case "k":
			if m.cursor > 0 {
				m.cursor--
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
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
	case tabMyTasks, tabTeam:
		return len(m.tasks)
	case tabViews:
		return len(m.cfg.Views)
	}
	return 0
}
