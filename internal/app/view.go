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

	// Indicator styles (grove pattern)
	overdueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")) // red
	dueSoonStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")) // yellow
	freshStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")) // green
	recentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")) // blue
	agingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")) // yellow
	staleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")) // red
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
	if m.showDetail {
		return m.renderDetail()
	}
	if m.showForm {
		return m.renderForm()
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

	// Breadcrumb (when navigated into a task)
	if len(m.navStack) > 0 {
		tabName := strings.SplitN(tabNames[m.activeTab], " [", 2)[0]
		crumbs := []string{dimStyle.Render(tabName)}
		for _, t := range m.navStack {
			crumbs = append(crumbs, titleStyle.Render(truncate(t.Title, 40)))
		}
		b.WriteString("  " + strings.Join(crumbs, dimStyle.Render(" › ")))
		b.WriteString("\n")
	}

	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")

	// Text filter input
	if m.filtering {
		b.WriteString(filterStyle.Render("  / " + m.filterQuery + "█"))
		b.WriteString("\n")
	}

	// Content
	switch m.activeTab {
	case tabMyTasks, tabTeam, tabDone:
		b.WriteString(m.renderTaskList(m.currentTasks()))
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
	case tabMyTasks, tabTeam, tabDone:
		n := len(m.currentTasks())
		label := "tasks"
		if len(m.navStack) > 0 {
			label = "subtasks"
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, label))
	case tabViews:
		parts = append(parts, fmt.Sprintf("%d views", len(m.cfg.Views)))
	}

	// Navigation hint
	if len(m.navStack) > 0 {
		parts = append(parts, "[/] sibling · esc back")
	}

	// Date scope and active field
	dateLabel := m.dateScope
	if m.dateField != "" && m.dateField != "updated" {
		dateLabel += " (" + m.dateField + ")"
	}
	parts = append(parts, dateLabel)

	// Hints
	parts = append(parts, "? help")
	parts = append(parts, "v"+m.version)
	if m.latestVersion != "" {
		parts = append(parts, "↑ "+m.latestVersion+" available")
	}

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
  f                        cycle date (today → yesterday → week → month → quarter → 6mo)
  F                        cycle date field (updated → created → start → target → closed)
  d                        cycle by assignee
  s                        cycle by type (feature, bug, user-story…)
  t                        cycle by tag / label
  esc                      clear all filters

Actions:
  c                        create work item
  enter                    navigate into task (show subtasks) · select view
  esc / backspace          navigate back · clear filters
  [ / ]                    prev / next sibling task
  i                        task detail overlay
  space                    copy task URL to clipboard
  o                        open task in browser
  r                        refresh data
  !                        about / paths
  ?                        this help
  q / ctrl+c               quit

Indicators:
  ⏱  due        ! overdue · ● due this week · ○ has date · — no date
  ↻  activity   ● green today · ● blue this week · ● yellow this month · ● red stale`

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

// ── Task detail overlay ─────────────────────────────────────────────────

func (m Model) renderDetail() string {
	t := m.detailTask
	w := min(80, m.width-4)

	row := func(label, value string) string {
		if value == "" {
			value = dimStyle.Render("—")
		}
		return fmt.Sprintf("  %-16s %s\n", dimStyle.Render(label), value)
	}

	ss := stateColors[t.State]

	var b strings.Builder
	b.WriteString(titleStyle.Render("  "+t.Title) + "\n\n")
	b.WriteString(row("ID", t.ID))
	b.WriteString(row("State", ss.Render(string(t.State))))
	b.WriteString(row("Type", string(t.Type)))
	b.WriteString(row("Assignee", t.Assignee))
	b.WriteString(row("Sprint", t.Sprint))
	b.WriteString(row("Parent", t.ParentTitle))
	if len(t.Labels) > 0 {
		b.WriteString(row("Tags", strings.Join(t.Labels, ", ")))
	} else {
		b.WriteString(row("Tags", ""))
	}
	b.WriteString("\n")
	b.WriteString(row("Created", fmtTime(t.CreatedAt)))
	b.WriteString(row("Updated", fmtTime(t.UpdatedAt)))
	b.WriteString(row("Start date", fmtTime(t.StartDate)))
	b.WriteString(row("Target date", fmtTime(t.TargetDate)))
	b.WriteString(row("Closed", fmtTime(t.ClosedAt)))
	b.WriteString(row("State changed", fmtTime(t.StateChangedAt)))
	if t.URL != "" {
		b.WriteString("\n")
		b.WriteString(row("URL", dimStyle.Render(t.URL)))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  [/] prev/next · enter navigate in · o open · space copy URL · esc close"))

	box := borderStyle.Width(w).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04") + dimStyle.Render(" ("+timeAgo(t)+")")
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
		if len(m.navStack) > 0 {
			return dimStyle.Render("  No subtasks.")
		}
		return dimStyle.Render("  No tasks found.")
	}

	var b strings.Builder

	// Header — indicators, then flex columns (title+parent), then fixed metadata.
	// Each column is separated by a single space for readability.
	// DUE(2) ACT(2) TITLE(flex 60%) PARENT(flex 40%) STATE(12) TYPE(12) ASSIGNEE(18) UPDATED(7) CREATED(7)
	const sep = " "
	fixedWidth := 2 + 2 + 12 + 12 + 18 + 7 + 7 // 60 (column widths)
	separators := 6                               // spaces between 7 columns (parent..created)
	flexWidth := m.width - fixedWidth - separators - 2 // 2 for prefix
	if flexWidth < 30 {
		flexWidth = 30
	}
	titleWidth := flexWidth * 3 / 5   // 60%
	parentWidth := flexWidth - titleWidth // 40%

	hdr := "  " +
		cell("!", 2) + cell("~", 2) +
		cell("TITLE", titleWidth) + sep +
		cell("PARENT", parentWidth) + sep +
		cell("STATE", 12) + sep +
		cell("TYPE", 12) + sep +
		cell("ASSIGNEE", 18) + sep +
		cell("UPDATED", 7) + sep +
		cell("CREATED", 7)
	b.WriteString(dimStyle.Render(hdr))
	b.WriteString("\n")

	for i, t := range tasks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		due := cell(dueIndicator(t), 2)
		act := cell(activityIndicator(t), 2)
		title := cell(truncate(t.Title, titleWidth), titleWidth)
		parent := dimStyle.Render(cell(truncate(t.ParentTitle, parentWidth), parentWidth))
		ss := stateColors[t.State]
		state := ss.Render(cell(string(t.State), 12))
		typ := typeStyle.Render(cell(string(t.Type), 12))
		assignee := dimStyle.Render(cell(truncate(t.Assignee, 18), 18))
		updated := dimStyle.Render(cell(timeAgo(t.UpdatedAt), 7))
		created := dimStyle.Render(cell(timeAgo(t.CreatedAt), 7))

		line := prefix + due + act + title + sep +
			parent + sep + state + sep + typ + sep +
			assignee + sep + updated + sep + created
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

// ── Indicator columns ──────────────────────────────────────────────────

// dueIndicator shows target-date urgency: ! overdue, ● due this week, ○ has date.
func dueIndicator(t model.Task) string {
	if t.TargetDate.IsZero() {
		return dimStyle.Render("—")
	}
	now := time.Now()
	if t.TargetDate.Before(now) && t.State != model.StateDone && t.State != model.StateClosed {
		return overdueStyle.Render("!")
	}
	if t.TargetDate.Before(now.AddDate(0, 0, 7)) {
		return dueSoonStyle.Render("●")
	}
	return dimStyle.Render("○")
}

// activityIndicator shows freshness based on last update: ● green/blue/yellow/red.
func activityIndicator(t model.Task) string {
	if t.UpdatedAt.IsZero() {
		return dimStyle.Render("—")
	}
	d := time.Since(t.UpdatedAt)
	switch {
	case d < 24*time.Hour:
		return freshStyle.Render("●")
	case d < 7*24*time.Hour:
		return recentStyle.Render("●")
	case d < 30*24*time.Hour:
		return agingStyle.Render("●")
	default:
		return staleStyle.Render("●")
	}
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
