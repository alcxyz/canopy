package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/alcxyz/canopy/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

var stateColors = map[model.TaskState]lipgloss.Style{
	model.StateTodo:       ui.StateTodoColor,
	model.StateInProgress: ui.StateInProgressColor,
	model.StateInReview:   ui.StateInReviewColor,
	model.StateDone:       ui.StateDoneColor,
	model.StateClosed:     ui.StateClosedColor,
}

func (m Model) View() string {
	if !m.ready {
		return "loading..."
	}

	// Overlays take over the full screen.
	if m.showHelp {
		return ui.RenderHelp(m.width, m.height)
	}
	if m.showSplash {
		return ui.RenderSplash(m.version, m.cfgPath, m.cacheDir, m.logPath, m.width, m.height)
	}
	if m.showDetail {
		return ui.RenderDetail(m.detailTask, stateColors, m.width, m.height)
	}
	if m.showForm {
		return m.renderForm()
	}

	var b strings.Builder

	// Title
	b.WriteString(ui.TitleStyle.Render("canopy"))
	b.WriteString("\n\n")

	// Tab bar
	b.WriteString(ui.RenderTabs(tabNames, int(m.activeTab), m.width))

	// Active filter indicator on same line after tabs
	if indicator := m.filterIndicator(); indicator != "" {
		b.WriteString("  ")
		b.WriteString(ui.FilterStyle.Render(indicator))
	}

	b.WriteString("\n")

	// Breadcrumb (when navigated into a task)
	if len(m.navStack) > 0 {
		tabName := tabNames[m.activeTab][2:] // strip "N " prefix
		crumbs := []string{ui.DimStyle.Render(tabName)}
		for _, t := range m.navStack {
			crumbs = append(crumbs, ui.TitleStyle.Render(truncate(t.Title, 40)))
		}
		b.WriteString("  " + strings.Join(crumbs, ui.DimStyle.Render(" › ")))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Text filter input
	if m.filtering {
		b.WriteString(ui.FilterStyle.Render("  / " + m.filterQuery + "█"))
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
	return ui.StatusStyle.Render(msg)
}

func (m Model) defaultStatusMsg() string {
	var parts []string
	if n := len(m.myTasks); n > 0 {
		parts = append(parts, ui.CountStyle.Render(fmt.Sprintf("%d", n))+" my")
	}
	if n := len(m.teamTasks); n > 0 {
		parts = append(parts, ui.CountStyle.Render(fmt.Sprintf("%d", n))+" team")
	}
	if n := len(m.doneTasks); n > 0 {
		parts = append(parts, ui.CountStyle.Render(fmt.Sprintf("%d", n))+" done")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

// ── Info bar ────────────────────────────────────────────────────────────

func (m Model) renderInfoBar() string {
	return ui.DimStyle.Render(m.infoBarText())
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

// ── Task list rendering ─────────────────────────────────────────────────

func (m Model) renderTaskList(tasks []model.Task) string {
	if len(tasks) == 0 {
		if len(m.backends) == 0 {
			return ui.DimStyle.Render("  No backends configured. Edit ~/.config/canopy/config.yaml")
		}
		if m.err != nil {
			return ui.DimStyle.Render(fmt.Sprintf("  Error: %v", m.err))
		}
		if len(m.navStack) > 0 {
			return ui.DimStyle.Render("  No subtasks.")
		}
		return ui.DimStyle.Render("  No tasks found.")
	}

	var b strings.Builder

	// Header — indicators, then flex columns (title+parent), then fixed metadata.
	// Each column is separated by a single space for readability.
	// DUE(2) ACT(2) TITLE(flex 60%) PARENT(flex 40%) STATE(12) TYPE(12) ASSIGNEE(18) UPDATED(7) CREATED(7)
	const sep = " "
	fixedWidth := 2 + 2 + 12 + 12 + 18 + 7 + 7         // 60 (column widths)
	separators := 6                                    // spaces between 7 columns (parent..created)
	flexWidth := m.width - fixedWidth - separators - 2 // 2 for prefix
	if flexWidth < 30 {
		flexWidth = 30
	}
	titleWidth := flexWidth * 3 / 5       // 60%
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
	b.WriteString(ui.DimStyle.Render(hdr))
	b.WriteString("\n")

	for i, t := range tasks {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		due := cell(dueIndicator(t), 2)
		act := cell(activityIndicator(t), 2)
		title := cell(truncate(t.Title, titleWidth), titleWidth)
		parent := ui.DimStyle.Render(cell(truncate(t.ParentTitle, parentWidth), parentWidth))
		ss := stateColors[t.State]
		state := ss.Render(cell(string(t.State), 12))
		typ := ui.TypeStyle.Render(cell(string(t.Type), 12))
		assignee := ui.DimStyle.Render(cell(truncate(t.Assignee, 18), 18))
		updated := ui.DimStyle.Render(cell(ui.TimeAgo(t.UpdatedAt), 7))
		created := ui.DimStyle.Render(cell(ui.TimeAgo(t.CreatedAt), 7))

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
		return ui.DimStyle.Render("  No views configured. Add views to your config.yaml")
	}

	var b strings.Builder
	for i, v := range m.cfg.Views {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		desc := ""
		if v.Description != "" {
			desc = ui.DimStyle.Render(" — " + v.Description)
		}

		line := fmt.Sprintf("%s%s%s", prefix, ui.TitleStyle.Render(v.Name), desc)
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
		return ui.DimStyle.Render("—")
	}
	now := time.Now()
	if t.TargetDate.Before(now) && t.State != model.StateDone && t.State != model.StateClosed {
		return ui.OverdueStyle.Render("!")
	}
	if t.TargetDate.Before(now.AddDate(0, 0, 7)) {
		return ui.DueSoonStyle.Render("●")
	}
	return ui.DimStyle.Render("○")
}

// activityIndicator shows freshness based on last update: ● green/blue/yellow/red.
func activityIndicator(t model.Task) string {
	if t.UpdatedAt.IsZero() {
		return ui.DimStyle.Render("—")
	}
	d := time.Since(t.UpdatedAt)
	switch {
	case d < 24*time.Hour:
		return ui.FreshStyle.Render("●")
	case d < 7*24*time.Hour:
		return ui.RecentStyle.Render("●")
	case d < 30*24*time.Hour:
		return ui.AgingStyle.Render("●")
	default:
		return ui.StaleStyle.Render("●")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────

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
