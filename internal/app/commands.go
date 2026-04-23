package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// ── Messages ────────────────────────────────────────────────────────────

type tasksLoadedMsg struct {
	myTasks   []model.Task
	teamTasks []model.Task
	doneTasks []model.Task
	err       error
}

type tickMsg time.Time
type ggTimeoutMsg struct{}
type versionCheckMsg struct{ latest string }

// ── Commands ────────────────────────────────────────────────────────────

// loadAllTasks fetches my tasks, team tasks (active only), and done tasks
// using the current dateScope for time-bounding.
func (m Model) loadAllTasks() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		days := dateScopeDays(m.dateScope)
		scope := fmt.Sprintf("last_%d_days", days)

		var myTasks, teamTasks, doneTasks []model.Task

		for _, b := range m.backends {
			// My active tasks (not done/closed)
			my, err := b.ListTasks(ctx, config.Filter{
				Assignee:     "me",
				Status:       []string{"todo", "in-progress", "in-review"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			myTasks = append(myTasks, my...)

			// Team active tasks (not done/closed)
			team, err := b.ListTasks(ctx, config.Filter{
				Status:       []string{"todo", "in-progress", "in-review"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			teamTasks = append(teamTasks, team...)

			// Done/closed tasks
			done, err := b.ListTasks(ctx, config.Filter{
				Status:       []string{"done", "closed"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			doneTasks = append(doneTasks, done...)
		}

		return tasksLoadedMsg{
			myTasks:   myTasks,
			teamTasks: teamTasks,
			doneTasks: doneTasks,
		}
	}
}

func (m Model) loadViewTasks(filter config.Filter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var tasks []model.Task
		for _, b := range m.backends {
			t, err := b.ListTasks(ctx, filter)
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			tasks = append(tasks, t...)
		}

		return tasksLoadedMsg{myTasks: tasks, teamTasks: tasks}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// isSemver returns true when v looks like a release version (x.y.z).
func isSemver(v string) bool {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// checkLatestVersion fetches the latest GitHub release tag in the background
// and returns a versionCheckMsg if a newer version is available.
// Silently no-ops for dev builds, hash builds, or when the network is unavailable.
func checkLatestVersion(version string) tea.Cmd {
	return func() tea.Msg {
		if !isSemver(version) {
			return versionCheckMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.github.com/repos/alcxyz/canopy/releases/latest", nil)
		if err != nil {
			return versionCheckMsg{}
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "canopy/"+version)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return versionCheckMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return versionCheckMsg{}
		}
		var payload struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return versionCheckMsg{}
		}
		if payload.TagName == "v"+version || payload.TagName == version {
			return versionCheckMsg{}
		}
		return versionCheckMsg{latest: payload.TagName}
	}
}
