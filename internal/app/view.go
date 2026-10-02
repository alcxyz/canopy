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
	switch {
	case m.showHelp:
		return ui.RenderHelp(m.width, m.height)
	case m.showSplash:
		return ui.RenderSplash(m.version, m.cfgPath, m.cacheDir, m.logPath, m.width, m.height)
	case m.showDetail:
		return ui.RenderDetail(m.detailTask, stateColors, m.width, m.height)
	case m.showForm:
		return m.renderForm()
	}

	var body string
	if m.showsTasks() {
		body = m.renderTaskList(m.currentTasks())
	} else {
		body = m.renderViews()
	}
	return m.renderHeader() + body + "\n" + m.renderFooter()
}

// renderHeader renders everything above the list. Every line, including the
// last, ends with a newline.
func (m Model) renderHeader() string {
	var b strings.Builder

	b.WriteString(ui.TitleStyle.Render("canopy"))
	b.WriteString("\n\n")

	b.WriteString(ui.RenderTabs(tabNames, int(m.activeTab), m.width))
	if indicator := m.filterIndicator(); indicator != "" {
		b.WriteString("  ")
		b.WriteString(ui.FilterStyle.Render(indicator))
	}
	b.WriteString("\n")

	// Breadcrumb (when inside a view or navigated into a task)
	if m.viewOpen() || len(m.navStack) > 0 {
		crumbs := []string{ui.DimStyle.Render(tabNames[m.activeTab][2:])} // strip "N " prefix
		if m.viewOpen() {
			crumbs = append(crumbs, ui.TitleStyle.Render(m.cfg.Views[m.viewIdx].Name))
		}
		for _, t := range m.navStack {
			crumbs = append(crumbs, ui.TitleStyle.Render(truncate(t.Title, 40)))
		}
		b.WriteString("  " + strings.Join(crumbs, ui.DimStyle.Render(" › ")))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.filtering {
		b.WriteString(ui.FilterStyle.Render("  / " + m.filterQuery + "█"))
		b.WriteString("\n")
	}
	return b.String()
}

// renderFooter renders the notice, status and info lines (footerLines lines,
// without a trailing newline).
func (m Model) renderFooter() string {
	lines := []string{m.renderStatusBar(), ui.DimStyle.Render(m.infoBarText())}
	if m.notice != "" {
		lines = append([]string{ui.WarnStyle.Render("⚠ " + m.notice)}, lines...)
	}
	return strings.Join(lines, "\n")
}

func (m Model) footerLines() int {
	if m.notice != "" {
		return 3
	}
	return 2
}

// visibleRows returns how many list rows fit between header and footer.
func (m Model) visibleRows() int {
	chrome := strings.Count(m.renderHeader(), "\n") + 1 + m.footerLines() // +1 blank line above footer
	if m.showsTasks() {
		chrome++ // column header
	}
	return max(1, m.height-chrome)
}

// ── Status bar ──────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	msg := m.statusMsg
	if m.loadingTasks || m.loadingView {
		msg = "⏳ " + msg
	}
	return ui.StatusStyle.Render(msg)
}

// ── Info bar ────────────────────────────────────────────────────────────

func (m Model) infoBarText() string {
	if m.filtering {
		return "type to filter · enter confirm · esc clear"
	}

	var parts []string

	n := m.listLen()
	label := "views"
	if m.showsTasks() {
		label = "tasks"
		if len(m.navStack) > 0 {
			label = "subtasks"
		}
	}
	if rows := m.visibleRows(); n > rows {
		parts = append(parts, fmt.Sprintf("%d–%d of %d %s", m.offset+1, min(n, m.offset+rows), n, label))
	} else {
		parts = append(parts, fmt.Sprintf("%d %s", n, label))
	}

	switch {
	case len(m.navStack) > 0:
		parts = append(parts, "[/] sibling · esc back")
	case m.viewOpen():
		parts = append(parts, "esc back to views")
	}

	if m.activeTab != tabViews {
		scope := "last " + pluralDays(m.scopeDays)
		if m.dateField() != "updated" {
			scope += " · dates: " + m.dateField()
		}
		parts = append(parts, scope)
		if !m.tasksLoadedAt.IsZero() {
			parts = append(parts, "synced "+ui.TimeAgo(m.tasksLoadedAt))
		}
	}

	parts = append(parts, "? help", "v"+m.version)
	if m.latestVersion != "" {
		parts = append(parts, "↑ "+m.latestVersion+" available")
	}

	return strings.Join(parts, " · ")
}

func pluralDays(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// ── Filter indicator ────────────────────────────────────────────────────

func (m Model) filterIndicator() string {
	var parts []string
	if m.filterQuery != "" {
		parts = append(parts, "/"+m.filterQuery)
	}
	if v, ok := m.activeCycleValue(m.cycleField); ok {
		parts = append(parts, m.cycleField+":"+v)
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
			return ui.DimStyle.Render("  No backends configured. Edit "+m.cfgPath) + "\n"
		}
		err := m.err
		if m.viewOpen() {
			err = m.viewErr
		}
		switch {
		case err != nil:
			return ui.DimStyle.Render(fmt.Sprintf("  Error: %v", err)) + "\n"
		case m.loadingTasks || m.loadingView:
			return ui.DimStyle.Render("  Loading…") + "\n"
		case len(m.navStack) > 0:
			return ui.DimStyle.Render("  No loaded subtasks.") + "\n"
		}
		return ui.DimStyle.Render("  No tasks found.") + "\n"
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

	end := min(len(tasks), m.offset+m.visibleRows())
	for i := m.offset; i < end; i++ {
		t := tasks[i]
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
		return ui.DimStyle.Render("  No views configured. Add views to "+m.cfgPath) + "\n"
	}

	var b strings.Builder
	end := min(len(m.cfg.Views), m.offset+m.visibleRows())
	for i := m.offset; i < end; i++ {
		v := m.cfg.Views[i]
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

// dueIndicator shows target-date urgency: ! overdue, ● due this week, ○ has
// date (or is already finished).
func dueIndicator(t model.Task) string {
	if t.TargetDate.IsZero() {
		return ui.DimStyle.Render("—")
	}
	if t.State == model.StateDone || t.State == model.StateClosed {
		return ui.DimStyle.Render("○")
	}
	now := time.Now()
	if t.TargetDate.Before(now) {
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
