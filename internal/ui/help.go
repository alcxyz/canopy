package ui

import "github.com/charmbracelet/lipgloss"

// RenderHelp renders the full-screen help overlay.
func RenderHelp(width, height int) string {
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

	box := BorderStyle.Width(min(72, width-4)).Render(
		TitleStyle.Render("canopy — help") + "\n\n" + help + "\n\n" +
			DimStyle.Render("press ? or esc to close"),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
