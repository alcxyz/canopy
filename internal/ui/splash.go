package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSplash renders the full-screen about/paths splash overlay.
func RenderSplash(version, cfgPath, cacheDir, logPath string, width, height int) string {
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
	info.WriteString(TitleStyle.Render(art))
	info.WriteString("\n\n")
	fmt.Fprintf(&info, "  %-14s %s\n", "version", version)
	fmt.Fprintf(&info, "  %-14s %s\n", "config", cfgPath)
	fmt.Fprintf(&info, "  %-14s %s\n", "cache", cacheDir)
	fmt.Fprintf(&info, "  %-14s %s\n", "log", logPath)
	info.WriteString("\n")
	info.WriteString(DimStyle.Render("  press ! or esc to close"))

	box := BorderStyle.Width(min(64, width-4)).Render(info.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
