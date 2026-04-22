package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
)

func (m Model) View() string {
	if !m.ready {
		return "loading..."
	}

	var b strings.Builder

	// Tab bar
	var tabs []string
	for i, name := range tabNames {
		if tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, tabStyle.Render(name))
		}
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n\n")

	// Content
	switch m.activeTab {
	case tabMyTasks:
		b.WriteString(m.renderTasks("my"))
	case tabTeam:
		b.WriteString(m.renderTasks("team"))
	case tabViews:
		b.WriteString(m.renderViews())
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("q quit • h/l tabs • j/k navigate"))

	return b.String()
}

func (m Model) renderTasks(scope string) string {
	if len(m.tasks) == 0 {
		if len(m.backends) == 0 {
			return dimStyle.Render("No backends configured. Edit ~/.config/canopy/config.yaml")
		}
		return dimStyle.Render("No tasks loaded. Backends are stubbed — implementation coming soon.")
	}

	var b strings.Builder
	for i, t := range m.tasks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-12s %-14s %-20s %s\n",
			prefix, t.State, t.Type, t.Assignee, t.Title))
	}
	return b.String()
}

func (m Model) renderViews() string {
	if len(m.cfg.Views) == 0 {
		return dimStyle.Render("No views configured. Add views to your config.yaml")
	}

	var b strings.Builder
	for i, v := range m.cfg.Views {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		desc := ""
		if v.Description != "" {
			desc = dimStyle.Render(" — " + v.Description)
		}
		b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, titleStyle.Render(v.Name), desc))
	}
	return b.String()
}
