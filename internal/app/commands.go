package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
	"github.com/alcxyz/canopy/internal/platform"
)

// loadTimeout bounds a whole refresh across all profiles.
const loadTimeout = 60 * time.Second

// ── Messages ────────────────────────────────────────────────────────────

// profileFailure records a profile whose query failed during a load.
type profileFailure struct {
	profile string
	err     error
}

type tasksLoadedMsg struct {
	seq       int
	days      int // history window the load covered
	myTasks   []model.Task
	teamTasks []model.Task
	doneTasks []model.Task
	failures  []profileFailure
	truncated bool // some results hit a backend's size cap
}

type viewLoadedMsg struct {
	seq       int
	tasks     []model.Task
	failures  []profileFailure
	truncated bool
}

type taskCreatedMsg struct {
	task model.Task
	err  error
}

type iterationResolvedMsg struct {
	formSeq int // form the lookup was made for
	path    string
	err     error
}

type openResultMsg struct{ err error }

type tickMsg time.Time
type ggTimeoutMsg struct{}
type versionCheckMsg struct{ latest string }

var (
	activeTaskStates = statusFilters(model.StateTodo, model.StateInProgress, model.StateInReview)
	doneTaskStates   = statusFilters(model.StateDone, model.StateClosed)
)

func statusFilters(states ...model.TaskState) []string {
	statuses := make([]string, len(states))
	for i, state := range states {
		statuses[i] = string(state)
	}
	return statuses
}

// ── Loading ─────────────────────────────────────────────────────────────

// startLoad requests a refresh of the task tabs. Any load still in flight is
// superseded. The history window is recomputed so an active date filter
// stays covered as days pass.
func (m *Model) startLoad() tea.Cmd {
	m.scopeDays = m.requiredScopeDays()
	m.loadSeq++
	m.loadingTasks = true
	return loadTasks(m.backends, m.loadSeq, m.scopeDays)
}

// startViewLoad requests the tasks for the open view.
func (m *Model) startViewLoad() tea.Cmd {
	m.viewSeq++
	m.loadingView = true
	return loadView(m.backends, m.viewSeq, m.cfg.Views[m.viewIdx].Filters)
}

// loadTasks fetches my, team and done tasks changed in the last days days.
func loadTasks(backends []backend.Backend, seq, days int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		defer cancel()

		since := fmt.Sprintf("last_%d_days", days)
		results := fetchAll(ctx, backends, []config.Filter{
			{Assignee: "me", Status: activeTaskStates, UpdatedSince: since},
			{Status: activeTaskStates, UpdatedSince: since},
			{Status: doneTaskStates, UpdatedSince: since},
		})

		msg := tasksLoadedMsg{seq: seq, days: days}
		for i, r := range results {
			if r.err != nil {
				msg.failures = append(msg.failures, profileFailure{backends[i].Name(), r.err})
				continue
			}
			msg.truncated = msg.truncated || r.truncated
			msg.myTasks = append(msg.myTasks, r.lists[0]...)
			msg.teamTasks = append(msg.teamTasks, r.lists[1]...)
			msg.doneTasks = append(msg.doneTasks, r.lists[2]...)
		}
		return msg
	}
}

// loadView fetches the tasks matching a configured view's filter.
func loadView(backends []backend.Backend, seq int, filter config.Filter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		defer cancel()

		msg := viewLoadedMsg{seq: seq}
		for i, r := range fetchAll(ctx, backends, []config.Filter{filter}) {
			if r.err != nil {
				msg.failures = append(msg.failures, profileFailure{backends[i].Name(), r.err})
				continue
			}
			msg.truncated = msg.truncated || r.truncated
			msg.tasks = append(msg.tasks, r.lists[0]...)
		}
		return msg
	}
}

// backendResult holds one task list per requested filter, or the first error
// the backend returned. Capped lists are kept and flagged as truncated.
type backendResult struct {
	lists     [][]model.Task
	err       error
	truncated bool
}

// fetchAll runs every filter against every backend concurrently. A failing
// backend does not affect the others.
func fetchAll(ctx context.Context, backends []backend.Backend, filters []config.Filter) []backendResult {
	results := make([]backendResult, len(backends))
	errs := make([][]error, len(backends))
	var wg sync.WaitGroup
	for i, b := range backends {
		results[i].lists = make([][]model.Task, len(filters))
		errs[i] = make([]error, len(filters))
		for j, f := range filters {
			wg.Go(func() {
				results[i].lists[j], errs[i][j] = b.ListTasks(ctx, f)
			})
		}
	}
	wg.Wait()

	for i := range results {
		for _, err := range errs[i] {
			switch {
			case err == nil:
			case errors.Is(err, backend.ErrTruncated):
				results[i].truncated = true
			case results[i].err == nil:
				results[i].err = err
			}
		}
	}
	return results
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ── Version check ───────────────────────────────────────────────────────

// parseSemver parses "x.y.z" or "vx.y.z" into its numeric parts.
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return out, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// isNewerVersion reports whether latest is a strictly newer release than current.
func isNewerVersion(latest, current string) bool {
	l, ok1 := parseSemver(latest)
	c, ok2 := parseSemver(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// checkLatestVersion fetches the latest GitHub release tag in the background
// and returns a versionCheckMsg if a newer version is available.
// Silently no-ops for dev builds or when the network is unavailable.
func checkLatestVersion(version string) tea.Cmd {
	return func() tea.Msg {
		if _, ok := parseSemver(version); !ok {
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
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return versionCheckMsg{}
		}
		var payload struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return versionCheckMsg{}
		}
		if !isNewerVersion(payload.TagName, version) {
			return versionCheckMsg{}
		}
		return versionCheckMsg{latest: payload.TagName}
	}
}

// ── Work item creation ──────────────────────────────────────────────────

// createTask submits the form to the backend it was opened for.
func createTask(f createForm) tea.Cmd {
	params := backend.CreateTaskParams{
		Type:               formTypes[f.typeIdx],
		Title:              strings.TrimSpace(f.values[formFieldTitle]),
		Description:        f.values[formFieldDesc],
		Iteration:          f.values[formFieldIteration],
		Assignee:           f.values[formFieldAssignee],
		Tags:               splitTags(f.values[formFieldTags]),
		StartDate:          f.values[formFieldStartDate],
		TargetDate:         f.values[formFieldTargetDate],
		AcceptanceCriteria: f.values[formFieldAcceptCriteria],
	}
	if f.parent != nil {
		params.ParentID = f.parent.ID
	}
	creator := f.creator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := creator.CreateTask(ctx, params)
		if err != nil {
			return taskCreatedMsg{err: err}
		}
		return taskCreatedMsg{task: result.Task}
	}
}

// splitTags parses comma-separated tags, trimming whitespace and dropping empties.
func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// resolveIteration fetches the current iteration path for form formSeq.
func resolveIteration(creator backend.TaskCreator, formSeq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		path, err := creator.CurrentIteration(ctx)
		return iterationResolvedMsg{formSeq: formSeq, path: path, err: err}
	}
}

// openURL opens url in the browser and reports whether the opener failed.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		return openResultMsg{err: platform.OpenURL(url)}
	}
}
