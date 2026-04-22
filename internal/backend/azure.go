package backend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

type azureBoards struct {
	profile config.Profile
	client  *http.Client
	token   string
	baseURL string // https://dev.azure.com/{org}/{project}
}

func newAzureBoards(p config.Profile) (Backend, error) {
	if p.Org == "" || p.Project == "" {
		return nil, fmt.Errorf("azure-boards: org and project are required")
	}
	token, err := resolveToken(p)
	if err != nil {
		return nil, fmt.Errorf("azure-boards: %w", err)
	}
	return &azureBoards{
		profile: p,
		client:  &http.Client{Timeout: 30 * time.Second},
		token:   token,
		baseURL: fmt.Sprintf("https://dev.azure.com/%s/%s", p.Org, p.Project),
	}, nil
}

func resolveToken(p config.Profile) (string, error) {
	if v := os.Getenv("AZURE_DEVOPS_PAT"); v != "" {
		return v, nil
	}
	if p.TokenFile != "" {
		data, err := os.ReadFile(p.TokenFile)
		if err != nil {
			return "", fmt.Errorf("reading token_file %q: %w", p.TokenFile, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("set AZURE_DEVOPS_PAT env var or token_file in config")
}

func (a *azureBoards) Name() string { return a.profile.Name }

func (a *azureBoards) authHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+a.token))
}

func (a *azureBoards) doRequest(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", a.authHeader())
	req.Header.Set("Content-Type", "application/json")

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

	query := buildWIQL(filter, a.profile.Team, iterPath)
	ids, err := a.queryWIQL(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	return a.fetchWorkItems(ctx, ids)
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

		url := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&$expand=links&api-version=7.0",
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
		CreatedAt: wi.Fields.CreatedDate,
		UpdatedAt: wi.Fields.ChangedDate,
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

// ── Azure DevOps response types ─────────────────────────────────────────

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
	Title         string        `json:"System.Title"`
	State         string        `json:"System.State"`
	WorkItemType  string        `json:"System.WorkItemType"`
	AssignedTo    assignedTo    `json:"System.AssignedTo"`
	Tags          string        `json:"System.Tags"`
	IterationPath string        `json:"System.IterationPath"`
	CreatedDate   time.Time     `json:"System.CreatedDate"`
	ChangedDate   time.Time     `json:"System.ChangedDate"`
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
