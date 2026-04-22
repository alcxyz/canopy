package app

import (
	"fmt"
	"strings"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/charmbracelet/lipgloss"
)

var (
	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	stateColors    = map[model.TaskState]lipgloss.Style{
		model.StateTodo:       lipgloss.NewStyle().Foreground(lipgloss.Color("#585b70")),
		model.StateInProgress: lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")),
		model.StateInReview:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")),
		model.StateDone:       lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")),
		model.StateClosed:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
	}
	typeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
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
	b.WriteString("\n")

	// Loading indicator
	if m.loading {
		b.WriteString(dimStyle.Render("  loading..."))
		b.WriteString("\n")
	}

	// Status / error
	if m.statusMsg != "" && !m.loading {
		b.WriteString(dimStyle.Render("  "+m.statusMsg))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Content
	switch m.activeTab {
	case tabMyTasks:
		b.WriteString(m.renderTaskList(m.myTasks))
	case tabTeam:
		b.WriteString(m.renderTaskList(m.teamTasks))
	case tabViews:
		b.WriteString(m.renderViews())
	}

	// Footer
	footer := "q quit • h/l tabs • j/k navigate • r refresh"
	if m.activeTab == tabMyTasks || m.activeTab == tabTeam {
		footer += " • o open"
	}
	if m.activeTab == tabViews {
		footer += " • enter select view"
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(footer))

	return b.String()
}

func (m Model) renderTaskList(tasks []model.Task) string {
	if len(tasks) == 0 {
		if len(m.backends) == 0 {
			return dimStyle.Render("  No backends configured. Edit ~/.config/canopy/config.yaml")
		}
		if m.err != nil {
			return dimStyle.Render(fmt.Sprintf("  Error: %v", m.err))
		}
		return dimStyle.Render("  No tasks found.")
	}

	var b strings.Builder

	// Header
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %-14s %-14s %-20s %s", "STATE", "TYPE", "ASSIGNEE", "TITLE")))
	b.WriteString("\n")

	for i, t := range tasks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		ss := stateColors[t.State]
		state := ss.Render(cell(string(t.State), 14))
		typ := typeStyle.Render(cell(string(t.Type), 14))
		assignee := cell(truncate(t.Assignee, 20), 20)
		title := truncate(t.Title, m.width-55)

		line := fmt.Sprintf("%s%s%s%s %s", prefix, state, typ, assignee, title)
		if i == m.cursor {
			line = selRow(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderViews() string {
	if len(m.cfg.Views) == 0 {
		return dimStyle.Render("  No views configured. Add views to your config.yaml")
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

		line := fmt.Sprintf("%s%s%s", prefix, titleStyle.Render(v.Name), desc)
		if i == m.cursor {
			line = selRow(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// ── helpers (mirrored from grove) ───────────────────────────────────────

func selRow(s string) string {
	const bg = "\033[48;2;69;71;90m"
	return bg + strings.ReplaceAll(s, "\033[0m", "\033[0m"+bg) + "\033[0m"
}

func cell(s string, w int) string {
	pad := w - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
