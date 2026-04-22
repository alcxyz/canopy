package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/charmbracelet/lipgloss"
)

var (
	tabStyle       = lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	filterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af"))
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9399b2"))
	countStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
	stateColors    = map[model.TaskState]lipgloss.Style{
		model.StateTodo:       lipgloss.NewStyle().Foreground(lipgloss.Color("#585b70")),
		model.StateInProgress: lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")),
		model.StateInReview:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")),
		model.StateDone:       lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")),
		model.StateClosed:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
	}
	typeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#585b70")).
			Padding(1, 2)
)

func (m Model) View() string {
	if !m.ready {
		return "loading..."
	}

	// Overlays take over the full screen.
	if m.showHelp {
		return m.renderHelp()
	}
	if m.showSplash {
		return m.renderSplash()
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

	// Active filter indicator next to tabs
	if indicator := m.filterIndicator(); indicator != "" {
		b.WriteString("  ")
		b.WriteString(filterStyle.Render(indicator))
	}

	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")

	// Text filter input
	if m.filtering {
		b.WriteString(filterStyle.Render("  / " + m.filterQuery + "█"))
		b.WriteString("\n")
	}

	// Content
	switch m.activeTab {
	case tabMyTasks:
		b.WriteString(m.renderTaskList(m.filteredTasks(m.myTasks)))
	case tabTeam:
		b.WriteString(m.renderTaskList(m.filteredTasks(m.teamTasks)))
	case tabDone:
		b.WriteString(m.renderTaskList(m.filteredTasks(m.doneTasks)))
	case tabViews:
		b.WriteString(m.renderViews())
	}

	// Status bar + info bar (bottom 2 lines)
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(m.renderInfoBar())

	return b.String()
}

// ── Status bar ──────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	msg := m.statusMsg
	if m.loading {
		msg = "⏳ " + msg
	}
	if msg == "" {
		msg = m.defaultStatusMsg()
	}
	return statusStyle.Render(msg)
}

func (m Model) defaultStatusMsg() string {
	var parts []string
	if n := len(m.myTasks); n > 0 {
		parts = append(parts, countStyle.Render(fmt.Sprintf("%d", n))+" my")
	}
	if n := len(m.teamTasks); n > 0 {
		parts = append(parts, countStyle.Render(fmt.Sprintf("%d", n))+" team")
	}
	if n := len(m.doneTasks); n > 0 {
		parts = append(parts, countStyle.Render(fmt.Sprintf("%d", n))+" done")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

// ── Info bar ────────────────────────────────────────────────────────────

func (m Model) renderInfoBar() string {
	return dimStyle.Render(m.infoBarText())
}

func (m Model) infoBarText() string {
	if m.filtering {
		return "type to filter · enter confirm · esc clear"
	}

	var parts []string

	// Tab-specific counts
	switch m.activeTab {
	case tabMyTasks:
		n := len(m.filteredTasks(m.myTasks))
		parts = append(parts, fmt.Sprintf("%d tasks", n))
	case tabTeam:
		n := len(m.filteredTasks(m.teamTasks))
		parts = append(parts, fmt.Sprintf("%d tasks", n))
	case tabDone:
		n := len(m.filteredTasks(m.doneTasks))
		parts = append(parts, fmt.Sprintf("%d tasks", n))
	case tabViews:
		parts = append(parts, fmt.Sprintf("%d views", len(m.cfg.Views)))
	}

	// Date scope
	parts = append(parts, m.dateScope)

	// Hints
	parts = append(parts, "? help")
	parts = append(parts, "v"+m.version)

	return strings.Join(parts, " · ")
}

// ── Filter indicator ────────────────────────────────────────────────────

func (m Model) filterIndicator() string {
	var parts []string
	if m.filterQuery != "" {
		parts = append(parts, fmt.Sprintf("/%s", m.filterQuery))
	}
	if m.cycleField != "" && m.cycleIdx >= 0 && m.cycleIdx < len(m.cycleValues) {
		parts = append(parts, fmt.Sprintf("%s:%s", m.cycleField, m.cycleValues[m.cycleIdx]))
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// ── Help overlay ────────────────────────────────────────────────────────

func (m Model) renderHelp() string {
	help := `Navigation:
  j / k                    move down / up
  gg / G                   first / last item
  h / l                    previous / next tab
  1–4                      switch to tab directly

Filters:
  /                        text search · esc clear
  f                        cycle date (this week → last week → month → quarter)
  d                        cycle by assignee
  s                        cycle by type (feature, bug, user-story…)
  esc                      clear all filters

Actions:
  o                        open task in browser
  r                        refresh data
  enter                    select view (Views tab)
  !                        about / paths
  ?                        this help
  q / ctrl+c               quit`

	box := borderStyle.Width(min(72, m.width-4)).Render(
		titleStyle.Render("canopy — help") + "\n\n" + help + "\n\n" +
			dimStyle.Render("press ? or esc to close"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ── Splash overlay ──────────────────────────────────────────────────────

func (m Model) renderSplash() string {
	art := `
     ╱╲
    ╱  ╲
   ╱ ╱╲ ╲
  ╱ ╱  ╲ ╲
 ╱ ╱    ╲ ╲
╱ ╱______╲ ╲
╲__________╱
  canopy`

	var info strings.Builder
	info.WriteString(titleStyle.Render(art))
	info.WriteString("\n\n")
	info.WriteString(fmt.Sprintf("  %-14s %s\n", "version", m.version))
	info.WriteString(fmt.Sprintf("  %-14s %s\n", "config", m.cfgPath))
	info.WriteString(fmt.Sprintf("  %-14s %s\n", "cache", m.cacheDir))
	info.WriteString(fmt.Sprintf("  %-14s %s\n", "log", m.logPath))
	info.WriteString("\n")
	info.WriteString(dimStyle.Render("  press ! or esc to close"))

	box := borderStyle.Width(min(64, m.width-4)).Render(info.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ── Task list rendering ─────────────────────────────────────────────────

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

	// Header — grove pattern: primary content first, metadata last (dimmed)
	// TITLE(flex) PARENT(20) STATE(12) TYPE(12) ASSIGNEE(16) UPDATED(7) CREATED(7)
	metaWidth := 20 + 12 + 12 + 16 + 7 + 7 // 74
	titleWidth := m.width - metaWidth - 3   // 2 prefix + 1 space
	if titleWidth < 16 {
		titleWidth = 16
	}

	b.WriteString(dimStyle.Render(fmt.Sprintf("  %-*s %-20s %-12s %-12s %-16s %-7s %-7s",
		titleWidth, "TITLE", "PARENT", "STATE", "TYPE", "ASSIGNEE", "UPDATED", "CREATED")))
	b.WriteString("\n")

	for i, t := range tasks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		title := cell(truncate(t.Title, titleWidth), titleWidth)
		parent := dimStyle.Render(cell(truncate(t.ParentTitle, 20), 20))
		ss := stateColors[t.State]
		state := ss.Render(cell(string(t.State), 12))
		typ := typeStyle.Render(cell(string(t.Type), 12))
		assignee := dimStyle.Render(cell(truncate(t.Assignee, 16), 16))
		updated := dimStyle.Render(cell(timeAgo(t.UpdatedAt), 7))
		created := dimStyle.Render(cell(timeAgo(t.CreatedAt), 7))

		line := fmt.Sprintf("%s%s %s%s%s%s%s%s", prefix, title, parent, state, typ, assignee, updated, created)
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

// ── helpers ─────────────────────────────────────────────────────────────

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}

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
