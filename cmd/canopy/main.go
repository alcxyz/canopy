package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/app"
	"github.com/alcxyz/canopy/internal/buildinfo"
	"github.com/alcxyz/canopy/internal/config"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
// Falls back to "dev" for local builds.
var version = "dev"

//go:embed config.example.yaml
var exampleConfig []byte

func setupLog() (string, func()) {
	logPath := config.LogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return "", func() {}
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return logPath, func() { _ = f.Close() }
}

const usage = `canopy — terminal UI for tracking tasks across work management backends

Usage:
  canopy                    launch the TUI
  canopy -v, --version      print version and paths
  canopy -h, --help         show this help

Navigation:
  1-4         switch tabs (My Tasks, Team, Done, Views)
  h/l         previous/next tab
  j/k         move cursor down/up
  enter       show subtasks · open view
  i           task details
  /           text filter
  ?           keybinding reference
  q           quit
`

func main() {
	currentVersion := buildinfo.Resolve(version)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "-help", "help", "h":
			fmt.Print(usage + "\nConfig: " + config.ConfigPath() + "\n")
			return
		case "-v", "--version", "-version", "version", "v":
			fmt.Printf("canopy %s\nconfig: %s\ncache:  %s\nlog:    %s\n",
				currentVersion, config.ConfigPath(), config.CacheDir(), config.LogPath())
			return
		default:
			fmt.Fprintf(os.Stderr, "canopy: unknown argument %q\n\n%s", os.Args[1], usage)
			os.Exit(2)
		}
	}

	logPath, closeLog := setupLog()
	defer closeLog()

	// First-run bootstrap: write the example config to the XDG path so the
	// user has a real file to edit rather than relying on compiled defaults.
	if config.NeedsBootstrap() {
		if p, err := config.BootstrapXDG(exampleConfig); err != nil {
			log.Printf("bootstrap config: %v", err)
		} else {
			log.Printf("bootstrapped config at %s", p)
		}
	}

	cfg, problems := config.Load()

	p := tea.NewProgram(
		app.New(app.Options{
			Cfg:      cfg,
			Problems: problems,
			Version:  currentVersion,
			LogPath:  logPath,
			CfgPath:  config.ConfigPath(),
			CacheDir: config.CacheDir(),
		}),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
