package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/app"
	"github.com/alcxyz/canopy/internal/config"
)

//go:embed config.example.yaml
var exampleConfig []byte

func main() {
	if config.NeedsBootstrap() {
		p, err := config.BootstrapXDG(exampleConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote example config to %s — edit it and re-run canopy.\n", p)
		os.Exit(0)
	}

	cfg := config.Load()

	logDir := filepath.Dir(config.LogPath())
	_ = os.MkdirAll(logDir, 0o755)
	logFile, err := os.OpenFile(config.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	p := tea.NewProgram(
		app.New(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
