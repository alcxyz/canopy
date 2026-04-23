// Headless smoke test — exercises the Azure Boards backend without the TUI.
// Run: go run ./cmd/smoke
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/config"
)

func main() {
	cfg := config.Load()
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(os.Stderr, "no profiles configured")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, p := range cfg.Profiles {
		b, err := backend.New(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profile %q: %v\n", p.Name, err)
			continue
		}

		fmt.Printf("=== Profile: %s ===\n\n", b.Name())

		// Sprints
		sprints, err := b.ListSprints(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  sprints error: %v\n", err)
		} else {
			fmt.Printf("Sprints (%d):\n", len(sprints))
			for _, s := range sprints {
				fmt.Printf("  %s  %s → %s\n", s.Name, s.StartDate.Format("2006-01-02"), s.EndDate.Format("2006-01-02"))
			}
		}

		// My tasks
		fmt.Println()
		myTasks, err := b.ListTasks(ctx, config.Filter{Assignee: "me"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  my tasks error: %v\n", err)
		} else {
			fmt.Printf("My Tasks (%d):\n", len(myTasks))
			for _, t := range myTasks {
				parent := ""
				if t.ParentTitle != "" {
					parent = fmt.Sprintf(" [parent: %s]", t.ParentTitle)
				}
				fmt.Printf("  [%s] %s — %s (%s)%s\n", t.State, t.ID, t.Title, t.Type, parent)
			}
		}

		// Team tasks (all)
		fmt.Println()
		teamTasks, err := b.ListTasks(ctx, config.Filter{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  team tasks error: %v\n", err)
		} else {
			fmt.Printf("Team Tasks (%d):\n", len(teamTasks))
			for _, t := range teamTasks {
				fmt.Printf("  [%s] %-12s %s — %s (%s)\n", t.State, t.Assignee, t.ID, t.Title, t.Type)
			}
		}
		fmt.Println()
	}
}
