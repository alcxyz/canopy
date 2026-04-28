package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
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

type azureBoards struct {
	profile config.Profile
	client  *http.Client
	baseURL string // https://dev.azure.com/{org}/{project}
}

func newAzureBoards(p config.Profile) (Backend, error) {
	if p.Org == "" || p.Project == "" {
		return nil, fmt.Errorf("azure-boards: org and project are required")
	}
	return &azureBoards{
		profile: p,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: fmt.Sprintf("https://dev.azure.com/%s/%s", p.Org, p.Project),
	}, nil
}

func (a *azureBoards) Name() string { return a.profile.Name }

// getAzToken acquires a short-lived Bearer token for Azure DevOps via the az CLI.
// The az CLI caches and refreshes tokens internally; no caching is needed here.
func getAzToken(ctx context.Context) (string, error) {
	const azureDevOpsResource = "499b84ac-1321-427f-aa17-267ca6975798"
	out, err := exec.CommandContext(ctx, "az", "account", "get-access-token",
		"--resource", azureDevOpsResource).Output()
	if err != nil {
		return "", fmt.Errorf("azure-boards: az CLI not found or not logged in — run 'az login' first")
	}
	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("azure-boards: failed to parse az token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("azure-boards: az returned empty access token — run 'az login' first")
	}
	return result.AccessToken, nil
}

func (a *azureBoards) doRequest(ctx context.Context, method, reqURL string, body io.Reader) ([]byte, error) {
	return a.doRequestCT(ctx, method, reqURL, body, "application/json")
}

func (a *azureBoards) doRequestCT(ctx context.Context, method, reqURL string, body io.Reader, contentType string) ([]byte, error) {
	acquire()
	defer release()

	token, err := getAzToken(ctx)
	if err != nil {
		return nil, err
	}

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
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// ── ListTasks ───────────────────────────────────────────────────────────

func (a *azureBoards) ListTasks(ctx context.Context, filter config.Filter) ([]model.Task, error) {
	// If filter needs current sprint, resolve the iteration path first.
	iterPath := ""
	if filter.Sprint == "current" {
		path, err := a.currentIterationPath(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolving current sprint: %w", err)
		}
		iterPath = path
	}

	query := buildWIQL(filter, a.profile.Project, a.profile.Team, iterPath)
	ids, err := a.queryWIQL(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	tasks, err := a.fetchWorkItems(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Resolve parent titles in a single batch.
	a.resolveParentTitles(ctx, tasks)

	return tasks, nil
}

func (a *azureBoards) queryWIQL(ctx context.Context, query string) ([]int, error) {
	payload := fmt.Sprintf(`{"query": %s}`, strconv.Quote(query))
	url := a.baseURL + "/_apis/wit/wiql?api-version=7.0&$top=200"
	data, err := a.doRequest(ctx, "POST", url, strings.NewReader(payload))
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

func (a *azureBoards) fetchWorkItems(ctx context.Context, ids []int) ([]model.Task, error) {
	var tasks []model.Task

	// Azure limits batch to 200 IDs per request.
	for i := 0; i < len(ids); i += 200 {
		end := i + 200
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		idStrs := make([]string, len(chunk))
		for j, id := range chunk {
			idStrs[j] = strconv.Itoa(id)
		}

		url := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&$expand=all&api-version=7.0",
			a.baseURL, strings.Join(idStrs, ","))
		data, err := a.doRequest(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("fetching work items: %w", err)
		}

		var resp workItemsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parsing work items: %w", err)
		}

		for _, wi := range resp.Value {
			tasks = append(tasks, a.mapWorkItem(wi))
		}
	}

	return tasks, nil
}

// resolveParentTitles batch-fetches parent work item titles and fills
// them into the tasks. Best-effort: errors are silently ignored.
func (a *azureBoards) resolveParentTitles(ctx context.Context, tasks []model.Task) {
	// Collect unique parent IDs that aren't already in the task set.
	taskIDs := map[string]bool{}
	for _, t := range tasks {
		taskIDs[t.ID] = true
	}
	parentIDs := map[string]bool{}
	for _, t := range tasks {
		if t.ParentID != "" && !taskIDs[t.ParentID] {
			parentIDs[t.ParentID] = true
		}
	}
	if len(parentIDs) == 0 {
		// All parents are in the task set already — resolve from there.
		titleMap := map[string]string{}
		for _, t := range tasks {
			titleMap[t.ID] = t.Title
		}
		for i := range tasks {
			if tasks[i].ParentID != "" {
				tasks[i].ParentTitle = titleMap[tasks[i].ParentID]
			}
		}
		return
	}

	// Fetch parent work items (just need titles).
	var ids []int
	for id := range parentIDs {
		if n, err := strconv.Atoi(id); err == nil {
			ids = append(ids, n)
		}
	}
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.Itoa(id)
	}

	titleMap := map[string]string{}
	// Also include titles from tasks we already have.
	for _, t := range tasks {
		titleMap[t.ID] = t.Title
	}

	// Batch fetch in chunks of 200.
	for i := 0; i < len(idStrs); i += 200 {
		end := i + 200
		if end > len(idStrs) {
			end = len(idStrs)
		}
		url := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&fields=System.Title&api-version=7.0",
			a.baseURL, strings.Join(idStrs[i:end], ","))
		data, err := a.doRequest(ctx, "GET", url, nil)
		if err != nil {
			continue // best-effort
		}
		var resp workItemsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		for _, wi := range resp.Value {
			titleMap[strconv.Itoa(wi.ID)] = wi.Fields.Title
		}
	}

	for i := range tasks {
		if tasks[i].ParentID != "" {
			tasks[i].ParentTitle = titleMap[tasks[i].ParentID]
		}
	}
}

func (a *azureBoards) mapWorkItem(wi workItem) model.Task {
	assignee := ""
	if wi.Fields.AssignedTo.DisplayName != "" {
		assignee = wi.Fields.AssignedTo.DisplayName
	}

	webURL := ""
	if wi.Links.HTML.Href != "" {
		webURL = wi.Links.HTML.Href
	}

	var labels []string
	if wi.Fields.Tags != "" {
		for _, t := range strings.Split(wi.Fields.Tags, ";") {
			t = strings.TrimSpace(t)
			if t != "" {
				labels = append(labels, t)
			}
		}
	}

	parentID := ""
	if wi.Fields.Parent > 0 {
		parentID = strconv.Itoa(wi.Fields.Parent)
	}

	return model.Task{
		ID:        strconv.Itoa(wi.ID),
		Title:     wi.Fields.Title,
		State:     mapAzureState(wi.Fields.State),
		Type:      mapAzureType(wi.Fields.WorkItemType),
		Assignee:  assignee,
		Labels:    labels,
		Sprint:    wi.Fields.IterationPath,
		URL:       webURL,
		Profile:   a.profile.Name,
		Backend:   string(config.BackendAzureBoards),
		ParentID:  parentID,
		CreatedAt:      wi.Fields.CreatedDate,
		UpdatedAt:      wi.Fields.ChangedDate,
		StartDate:      wi.Fields.StartDate,
		TargetDate:     wi.Fields.TargetDate,
		ClosedAt:       wi.Fields.ClosedDate,
		StateChangedAt: wi.Fields.StateChangeDate,
	}
}

// ── ListSprints ─────────────────────────────────────────────────────────

func (a *azureBoards) ListSprints(ctx context.Context) ([]model.Sprint, error) {
	team := a.teamName()
	url := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations?api-version=7.0",
		a.baseURL, team)
	data, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("listing iterations: %w", err)
	}

	var resp iterationsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing iterations: %w", err)
	}

	sprints := make([]model.Sprint, len(resp.Value))
	for i, it := range resp.Value {
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

func (a *azureBoards) currentIterationPath(ctx context.Context) (string, error) {
	team := a.teamName()
	url := fmt.Sprintf("%s/%s/_apis/work/teamsettings/iterations?$timeframe=current&api-version=7.0",
		a.baseURL, team)
	data, err := a.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	var resp iterationsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Value) == 0 {
		return "", fmt.Errorf("no current iteration found")
	}
	return resp.Value[0].Path, nil
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
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
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
	ID     int           `json:"id"`
	Fields workItemFields `json:"fields"`
	Links  struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"_links"`
}

type workItemFields struct {
	Title            string     `json:"System.Title"`
	State            string     `json:"System.State"`
	WorkItemType     string     `json:"System.WorkItemType"`
	AssignedTo       assignedTo `json:"System.AssignedTo"`
	Tags             string     `json:"System.Tags"`
	IterationPath    string     `json:"System.IterationPath"`
	Parent           int        `json:"System.Parent"`
	CreatedDate      time.Time  `json:"System.CreatedDate"`
	ChangedDate      time.Time  `json:"System.ChangedDate"`
	StartDate        time.Time  `json:"Microsoft.VSTS.Scheduling.StartDate"`
	TargetDate       time.Time  `json:"Microsoft.VSTS.Scheduling.TargetDate"`
	ClosedDate       time.Time  `json:"Microsoft.VSTS.Common.ClosedDate"`
	StateChangeDate  time.Time  `json:"Microsoft.VSTS.Common.StateChangeDate"`
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
	} `json:"attributes"`
}
