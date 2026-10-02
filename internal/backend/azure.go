package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// sem is a counting semaphore that limits concurrent Azure DevOps API calls
// to 5. Azure DevOps has a per-user rate limit; capping concurrency prevents
// exhausting the budget when multiple profiles or refreshes overlap.
var sem = make(chan struct{}, 5)

func acquire() { sem <- struct{}{} }
func release() { <-sem }

// wiqlTop caps the number of work item IDs returned by one WIQL query.
const wiqlTop = 200

// workItemBatchSize is the Azure DevOps limit for IDs per work item request.
const workItemBatchSize = 200

type azureBoards struct {
	profile config.Profile
	client  *http.Client
	baseURL string // https://dev.azure.com/{org}/{project}
	token   func(context.Context) (string, error)
}

func newAzureBoards(p config.Profile) (Backend, error) {
	if p.Org == "" || p.Project == "" {
		return nil, fmt.Errorf("azure-boards: org and project are required")
	}
	return &azureBoards{
		profile: p,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: fmt.Sprintf("https://dev.azure.com/%s/%s", url.PathEscape(p.Org), url.PathEscape(p.Project)),
		token:   azTokens.Token,
	}, nil
}

func (a *azureBoards) Name() string { return a.profile.Name }

func (a *azureBoards) doRequest(ctx context.Context, method, reqURL string, body io.Reader) ([]byte, error) {
	return a.doRequestCT(ctx, method, reqURL, body, "application/json")
}

func (a *azureBoards) doRequestCT(ctx context.Context, method, reqURL string, body io.Reader, contentType string) ([]byte, error) {
	// Resolve the token before taking a request slot so a slow az CLI call
	// does not hold up other requests.
	token, err := a.token(ctx)
	if err != nil {
		return nil, err
	}

	acquire()
	defer release()

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpError(resp.StatusCode, data)
	}
	return data, nil
}

// httpError summarises a failed response. Azure DevOps usually returns JSON
// with a message field; anything else (such as an HTML sign-in page) is
// truncated so it fits the status bar.
func httpError(status int, body []byte) error {
	var apiErr struct {
		Message string `json:"message"`
	}
	msg := ""
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		msg = apiErr.Message
	} else {
		msg = strings.Join(strings.Fields(string(body)), " ")
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("HTTP %d: %s", status, truncateText(msg, 200))
}

// ── ListTasks ───────────────────────────────────────────────────────────

func (a *azureBoards) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	iterPath, err := a.resolveIterationPath(ctx, filter.Sprint)
	if err != nil {
		return nil, fmt.Errorf("resolving sprint %q: %w", filter.Sprint, err)
	}

	query := buildWIQL(filter, a.profile.Project, a.profile.Team, iterPath)
	ids, err := a.queryWIQL(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) >= wiqlTop {
		log.Printf("azure-boards %s: query returned the %d-item cap; results are truncated", a.profile.Name, wiqlTop)
	}

	items, err := a.getWorkItems(ctx, ids, "$expand=all")
	if err != nil {
		return nil, fmt.Errorf("fetching work items: %w", err)
	}
	tasks := make([]model.Task, len(items))
	for i, wi := range items {
		tasks[i] = a.mapWorkItem(wi)
	}

	a.resolveParentTitles(ctx, tasks)
	return tasks, nil
}

func (a *azureBoards) queryWIQL(ctx context.Context, query string) ([]int, error) {
	payload, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/_apis/wit/wiql?api-version=7.0&$top=%d", a.baseURL, wiqlTop)
	data, err := a.doRequest(ctx, "POST", reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("WIQL query: %w", err)
	}

	var resp wiqlResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing WIQL response: %w", err)
	}

	ids := make([]int, len(resp.WorkItems))
	for i, wi := range resp.WorkItems {
		ids[i] = wi.ID
	}
	return ids, nil
}

// getWorkItems fetches work items by ID in batches. params is appended to the
// query string (for example "$expand=all" or "fields=System.Title"). On error
// it returns the items fetched so far alongside the error.
func (a *azureBoards) getWorkItems(ctx context.Context, ids []int, params string) ([]workItem, error) {
	var items []workItem
	for start := 0; start < len(ids); start += workItemBatchSize {
		chunk := ids[start:min(start+workItemBatchSize, len(ids))]
		idStrs := make([]string, len(chunk))
		for i, id := range chunk {
			idStrs[i] = strconv.Itoa(id)
		}

		reqURL := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&%s&api-version=7.0",
			a.baseURL, strings.Join(idStrs, ","), params)
		data, err := a.doRequest(ctx, "GET", reqURL, nil)
		if err != nil {
			return items, err
		}
		var resp workItemsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return items, fmt.Errorf("parsing work items: %w", err)
		}
		items = append(items, resp.Value...)
	}
	return items, nil
}

// resolveParentTitles fills ParentTitle, fetching titles for parents that are
// not part of the task set. Best-effort: failures leave titles empty.
func (a *azureBoards) resolveParentTitles(ctx context.Context, tasks []model.Task) {
	titles := make(map[string]string, len(tasks))
	for _, t := range tasks {
		titles[t.ID] = t.Title
	}

	var missing []int
	requested := map[string]bool{}
	for _, t := range tasks {
		if t.ParentID == "" || requested[t.ParentID] {
			continue
		}
		if _, ok := titles[t.ParentID]; ok {
			continue
		}
		requested[t.ParentID] = true
		if id, err := strconv.Atoi(t.ParentID); err == nil {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		parents, err := a.getWorkItems(ctx, missing, "fields=System.Title")
		if err != nil {
			log.Printf("azure-boards %s: resolving parent titles: %v", a.profile.Name, err)
		}
		for _, wi := range parents {
			titles[strconv.Itoa(wi.ID)] = wi.Fields.Title
		}
	}

	for i := range tasks {
		if tasks[i].ParentID != "" {
			tasks[i].ParentTitle = titles[tasks[i].ParentID]
		}
	}
}

func (a *azureBoards) mapWorkItem(wi workItem) model.Task {
	var labels []string
	for _, t := range strings.Split(wi.Fields.Tags, ";") {
		if t = strings.TrimSpace(t); t != "" {
			labels = append(labels, t)
		}
	}

	parentID := ""
	if wi.Fields.Parent > 0 {
		parentID = strconv.Itoa(wi.Fields.Parent)
	}

	return model.Task{
		ID:             strconv.Itoa(wi.ID),
		Title:          wi.Fields.Title,
		State:          mapAzureState(wi.Fields.State),
		Type:           mapAzureType(wi.Fields.WorkItemType),
		Assignee:       wi.Fields.AssignedTo.DisplayName,
		Labels:         labels,
		Sprint:         wi.Fields.IterationPath,
		URL:            wi.Links.HTML.Href,
		Profile:        a.profile.Name,
		Backend:        string(config.BackendAzureBoards),
		ParentID:       parentID,
		CreatedAt:      wi.Fields.CreatedDate,
		UpdatedAt:      wi.Fields.ChangedDate,
		StartDate:      wi.Fields.StartDate,
		TargetDate:     wi.Fields.TargetDate,
		ClosedAt:       wi.Fields.ClosedDate,
		StateChangedAt: wi.Fields.StateChangeDate,
	}
}

// ── Sprints ─────────────────────────────────────────────────────────────

func (a *azureBoards) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	its, err := a.teamIterations(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("listing iterations: %w", err)
	}

	sprints := make([]model.Sprint, len(its))
	for i, it := range its {
		sprints[i] = model.Sprint{
			ID:        it.ID,
			Name:      it.Name,
			StartDate: it.Attributes.StartDate,
			EndDate:   it.Attributes.FinishDate,
			Profile:   a.profile.Name,
		}
	}
	return sprints, nil
}

// teamIterations lists the team's iterations, optionally restricted to a
// timeframe ("current").
func (a *azureBoards) teamIterations(ctx context.Context, timeframe string) ([]iteration, error) {
	reqURL := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations?api-version=7.0",
		a.baseURL, url.PathEscape(a.teamName()))
	if timeframe != "" {
		reqURL += "&$timeframe=" + url.QueryEscape(timeframe)
	}
	data, err := a.doRequest(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var resp iterationsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing iterations: %w", err)
	}
	return resp.Value, nil
}

func (a *azureBoards) currentIterationPath(ctx context.Context) (string, error) {
	its, err := a.teamIterations(ctx, "current")
	if err != nil {
		return "", err
	}
	if len(its) == 0 {
		return "", fmt.Errorf("no current iteration found")
	}
	return its[0].Path, nil
}

// resolveIterationPath maps a filter sprint value to an iteration path:
// "current" and "previous" are resolved from the team's iterations, a value
// containing a backslash is used as a full path, and anything else is matched
// against iteration names.
func (a *azureBoards) resolveIterationPath(ctx context.Context, sprint string) (string, error) {
	switch {
	case sprint == "":
		return "", nil
	case sprint == "current":
		return a.currentIterationPath(ctx)
	case strings.Contains(sprint, `\`):
		return sprint, nil
	}

	its, err := a.teamIterations(ctx, "")
	if err != nil {
		return "", err
	}
	return matchIteration(its, sprint)
}

// matchIteration finds the iteration path for a sprint name or "previous"
// (the most recently finished iteration).
func matchIteration(its []iteration, sprint string) (string, error) {
	if sprint == "previous" {
		var prev *iteration
		for i := range its {
			it := &its[i]
			if it.Attributes.TimeFrame != "past" {
				continue
			}
			if prev == nil || it.Attributes.FinishDate.After(prev.Attributes.FinishDate) {
				prev = it
			}
		}
		if prev == nil {
			return "", fmt.Errorf("no previous iteration found")
		}
		return prev.Path, nil
	}

	for _, it := range its {
		if strings.EqualFold(it.Name, sprint) {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("no team iteration named %q", sprint)
}

func (a *azureBoards) teamName() string {
	if a.profile.AzureTeam != "" {
		return a.profile.AzureTeam
	}
	return a.profile.Project + " Team"
}

// ── ListTeam ────────────────────────────────────────────────────────────

func (a *azureBoards) ListTeam(_ context.Context) ([]model.TeamMember, error) {
	members := make([]model.TeamMember, len(a.profile.Team))
	for i, t := range a.profile.Team {
		members[i] = model.TeamMember{
			ID:      t,
			Name:    t,
			Email:   t,
			Profile: a.profile.Name,
		}
	}
	return members, nil
}

// ── CreateTask ──────────────────────────────────────────────────────────

// CurrentIteration returns the current sprint iteration path.
func (a *azureBoards) CurrentIteration(ctx context.Context) (string, error) {
	return a.currentIterationPath(ctx)
}

// buildCreateOps constructs the JSON patch operations for creating a work item.
// It is extracted for testability.
func buildCreateOps(params CreateTaskParams, orgName string) []jsonPatchOp {
	ops := []jsonPatchOp{
		{Op: "add", Path: "/fields/System.Title", Value: params.Title},
	}
	if params.DescriptionHTML != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/System.Description",
			Value: params.DescriptionHTML,
		})
	} else if params.Description != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/System.Description",
			Value: plainTextToHTML(params.Description),
		})
	}
	if params.Iteration != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/System.IterationPath",
			Value: params.Iteration,
		})
	}
	if params.Assignee != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/System.AssignedTo",
			Value: params.Assignee,
		})
	}
	if params.ParentID != "" {
		parentURL := fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/workItems/%s",
			orgName, params.ParentID)
		ops = append(ops, jsonPatchOp{
			Op:   "add",
			Path: "/relations/-",
			Value: relationValue{
				Rel: "System.LinkTypes.Hierarchy-Reverse",
				URL: parentURL,
			},
		})
	}
	if len(params.Tags) > 0 {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/System.Tags",
			Value: strings.Join(params.Tags, "; "),
		})
	}
	if params.StartDate != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.StartDate",
			Value: params.StartDate,
		})
	}
	if params.TargetDate != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.TargetDate",
			Value: params.TargetDate,
		})
	}
	if params.AcceptanceCriteria != "" {
		ops = append(ops, jsonPatchOp{
			Op: "add", Path: "/fields/Microsoft.VSTS.Common.AcceptanceCriteria",
			Value: plainTextToHTML(params.AcceptanceCriteria),
		})
	}
	return ops
}

// CreateTask creates a new work item in Azure DevOps.
func (a *azureBoards) CreateTask(ctx context.Context, params CreateTaskParams) (CreateTaskResult, error) {
	azType, ok := typeToAzure[params.Type]
	if !ok {
		return CreateTaskResult{}, fmt.Errorf("unsupported work item type: %s", params.Type)
	}

	ops := buildCreateOps(params, a.profile.Org)

	body, err := json.Marshal(ops)
	if err != nil {
		return CreateTaskResult{}, fmt.Errorf("marshalling patch document: %w", err)
	}

	reqURL := fmt.Sprintf("%s/_apis/wit/workitems/$%s?api-version=7.0",
		a.baseURL, url.PathEscape(azType))

	data, err := a.doRequestCT(ctx, "PATCH", reqURL, bytes.NewReader(body),
		"application/json-patch+json")
	if err != nil {
		return CreateTaskResult{}, fmt.Errorf("creating work item: %w", err)
	}

	var wi workItem
	if err := json.Unmarshal(data, &wi); err != nil {
		return CreateTaskResult{}, fmt.Errorf("parsing created work item: %w", err)
	}

	return CreateTaskResult{Task: a.mapWorkItem(wi)}, nil
}

// ── Azure DevOps response types ─────────────────────────────────────────

type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type relationValue struct {
	Rel string `json:"rel"`
	URL string `json:"url"`
}

func plainTextToHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\n", "<br>\n")
	return s
}

type wiqlResponse struct {
	WorkItems []struct {
		ID int `json:"id"`
	} `json:"workItems"`
}

type workItemsResponse struct {
	Value []workItem `json:"value"`
}

type workItem struct {
	ID     int            `json:"id"`
	Fields workItemFields `json:"fields"`
	Links  struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"_links"`
}

type workItemFields struct {
	Title           string     `json:"System.Title"`
	State           string     `json:"System.State"`
	WorkItemType    string     `json:"System.WorkItemType"`
	AssignedTo      assignedTo `json:"System.AssignedTo"`
	Tags            string     `json:"System.Tags"`
	IterationPath   string     `json:"System.IterationPath"`
	Parent          int        `json:"System.Parent"`
	CreatedDate     time.Time  `json:"System.CreatedDate"`
	ChangedDate     time.Time  `json:"System.ChangedDate"`
	StartDate       time.Time  `json:"Microsoft.VSTS.Scheduling.StartDate"`
	TargetDate      time.Time  `json:"Microsoft.VSTS.Scheduling.TargetDate"`
	ClosedDate      time.Time  `json:"Microsoft.VSTS.Common.ClosedDate"`
	StateChangeDate time.Time  `json:"Microsoft.VSTS.Common.StateChangeDate"`
}

type assignedTo struct {
	DisplayName string `json:"displayName"`
	UniqueName  string `json:"uniqueName"`
}

type iterationsResponse struct {
	Value []iteration `json:"value"`
}

type iteration struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Attributes struct {
		StartDate  time.Time `json:"startDate"`
		FinishDate time.Time `json:"finishDate"`
		TimeFrame  string    `json:"timeFrame"` // past, current or future
	} `json:"attributes"`
}
