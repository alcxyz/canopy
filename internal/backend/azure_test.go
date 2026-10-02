package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alcxyz/canopy/internal/config"
)

// staticToken is a tokenSource that always returns the same token.
type staticToken struct {
	token       string
	invalidated []string
}

func (s *staticToken) Token(context.Context) (string, error) { return s.token, nil }
func (s *staticToken) Invalidate(token string)               { s.invalidated = append(s.invalidated, token) }

// newTestAzure returns an Azure backend pointed at a test server.
func newTestAzure(t *testing.T, handler http.HandlerFunc) *azureBoards {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &azureBoards{
		profile: config.Profile{Name: "Work", Org: "org", Project: "proj"},
		client:  srv.Client(),
		baseURL: srv.URL + "/org/proj",
		tokens:  &staticToken{token: "test-token"},
	}
}

func TestListTasks_FetchesItemsAndParentTitles(t *testing.T) {
	var wiql string
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		switch {
		case r.URL.Path == "/org/proj/_apis/wit/wiql":
			var body struct{ Query string }
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			wiql = body.Query
			_, _ = io.WriteString(w, `{"workItems":[{"id":1},{"id":2}]}`)
		case r.URL.Path == "/org/proj/_apis/wit/workitems" && r.URL.Query().Get("ids") == "1,2":
			_, _ = io.WriteString(w, `{"value":[
				{"id":1,"fields":{"System.Title":"Story","System.State":"In Progress","System.WorkItemType":"User Story","System.Tags":"a; b"},
				 "_links":{"html":{"href":"https://example/1"}}},
				{"id":2,"fields":{"System.Title":"Task","System.State":"New","System.WorkItemType":"Task","System.Parent":99}}
			]}`)
		case r.URL.Path == "/org/proj/_apis/wit/workitems" && r.URL.Query().Get("ids") == "99":
			if r.URL.Query().Get("fields") != "System.Title" {
				t.Errorf("parent lookup should only request titles, got %q", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"value":[{"id":99,"fields":{"System.Title":"Feature"}}]}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})

	tasks, err := a.ListTasks(context.Background(), config.Filter{Assignee: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wiql, "[System.AssignedTo] = @me") {
		t.Errorf("WIQL missing assignee clause: %s", wiql)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].State != "in-progress" || tasks[0].URL != "https://example/1" || len(tasks[0].Labels) != 2 {
		t.Errorf("task 1 mapped incorrectly: %+v", tasks[0])
	}
	if tasks[1].ParentID != "99" || tasks[1].ParentTitle != "Feature" {
		t.Errorf("task 2 parent = %q/%q, want 99/Feature", tasks[1].ParentID, tasks[1].ParentTitle)
	}
}

func TestListTasks_ResolvesSprintName(t *testing.T) {
	var wiql string
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/proj/proj Team/_apis/work/teamsettings/iterations":
			_, _ = io.WriteString(w, `{"value":[
				{"name":"Sprint 4","path":"proj\\Sprint 4","attributes":{"timeFrame":"past"}},
				{"name":"Sprint 5","path":"proj\\Sprint 5","attributes":{"timeFrame":"current"}}
			]}`)
		case "/org/proj/_apis/wit/wiql":
			var body struct{ Query string }
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			wiql = body.Query
			_, _ = io.WriteString(w, `{"workItems":[]}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	})

	if _, err := a.ListTasks(context.Background(), config.Filter{Sprint: "sprint 5"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wiql, `[System.IterationPath] = 'proj\Sprint 5'`) {
		t.Errorf("WIQL did not use resolved path: %s", wiql)
	}
}

func TestDoRequest_SummarisesErrors(t *testing.T) {
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"message":"TF51005: bad field","typeKey":"X"}`)
	})
	_, err := a.doRequest(context.Background(), "GET", a.baseURL+"/x", nil)
	if err == nil || err.Error() != "HTTP 400: TF51005: bad field" {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPError_TruncatesNonJSON(t *testing.T) {
	err := httpError(401, []byte("<html>\n"+strings.Repeat("x", 500)+"</html>"))
	if got := err.Error(); len([]rune(got)) > 220 || !strings.HasPrefix(got, "HTTP 401: <html> x") {
		t.Errorf("unexpected error text: %q", got)
	}
}

func TestMatchIteration(t *testing.T) {
	its := make([]iteration, 3)
	its[0].Name, its[0].Path, its[0].Attributes.TimeFrame = "Sprint 1", `p\Sprint 1`, "past"
	its[0].Attributes.FinishDate = time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	its[1].Name, its[1].Path, its[1].Attributes.TimeFrame = "Sprint 2", `p\Sprint 2`, "past"
	its[1].Attributes.FinishDate = time.Date(2026, 1, 28, 0, 0, 0, 0, time.UTC)
	its[2].Name, its[2].Path, its[2].Attributes.TimeFrame = "Sprint 3", `p\Sprint 3`, "current"

	if got, err := matchIteration(its, "previous"); err != nil || got != `p\Sprint 2` {
		t.Errorf("previous = %q, %v", got, err)
	}
	if got, err := matchIteration(its, "SPRINT 1"); err != nil || got != `p\Sprint 1` {
		t.Errorf("by name = %q, %v", got, err)
	}
	if got, err := matchIteration(its, "MyProject"); err != nil || got != "MyProject" {
		t.Errorf("unknown name should be used as a path, got %q, %v", got, err)
	}
}

func TestParseAzToken(t *testing.T) {
	token, exp, err := parseAzToken([]byte(`{"accessToken":"abc","expiresOn":"2026-10-02 12:00:00.000000","expires_on":1790000000}`))
	if err != nil || token != "abc" {
		t.Fatalf("token = %q, err = %v", token, err)
	}
	if !exp.Equal(time.Unix(1790000000, 0)) {
		t.Errorf("expires = %v, want unix timestamp to win", exp)
	}

	_, exp, err = parseAzToken([]byte(`{"accessToken":"abc","expiresOn":"2026-10-02 12:00:00.000000"}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local); !exp.Equal(want) {
		t.Errorf("expires = %v, want %v", exp, want)
	}

	if _, _, err := parseAzToken([]byte(`{"accessToken":""}`)); err == nil {
		t.Error("expected error for empty token")
	}
}

func TestCacheUntil(t *testing.T) {
	now := time.Now()
	if got := cacheUntil(time.Time{}, now); !got.Equal(now.Add(azTokenFallbackTTL)) {
		t.Errorf("unknown expiry: got %v", got)
	}
	exp := now.Add(time.Hour)
	if got := cacheUntil(exp, now); !got.Equal(exp.Add(-azTokenMargin)) {
		t.Errorf("known expiry: got %v", got)
	}
}

func TestListTasks_ReportsTruncation(t *testing.T) {
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/proj/_apis/wit/wiql":
			ids := make([]string, wiqlTop+1)
			for i := range ids {
				ids[i] = fmt.Sprintf(`{"id":%d}`, i+1)
			}
			_, _ = io.WriteString(w, `{"workItems":[`+strings.Join(ids, ",")+`]}`)
		case "/org/proj/_apis/wit/workitems":
			var items []string
			for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
				items = append(items, `{"id":`+id+`,"fields":{"System.Title":"t"}}`)
			}
			_, _ = io.WriteString(w, `{"value":[`+strings.Join(items, ",")+`]}`)
		}
	})

	tasks, err := a.ListTasks(context.Background(), config.Filter{})
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
	if len(tasks) != wiqlTop {
		t.Errorf("got %d tasks, want %d", len(tasks), wiqlTop)
	}
}

func TestListTasks_ExactlyCapIsNotTruncated(t *testing.T) {
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/proj/_apis/wit/wiql":
			if got := r.URL.Query().Get("$top"); got != fmt.Sprint(wiqlTop+1) {
				t.Errorf("$top = %s", got)
			}
			_, _ = io.WriteString(w, `{"workItems":[{"id":1}]}`)
		case "/org/proj/_apis/wit/workitems":
			_, _ = io.WriteString(w, `{"value":[{"id":1,"fields":{"System.Title":"t"}}]}`)
		}
	})
	if _, err := a.ListTasks(context.Background(), config.Filter{}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestDoRequest_InvalidatesRejectedToken(t *testing.T) {
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, _ = a.doRequest(context.Background(), "GET", a.baseURL+"/x", nil)
	if got := a.tokens.(*staticToken).invalidated; len(got) != 1 || got[0] != "test-token" {
		t.Errorf("invalidated = %q", got)
	}
}

func TestHTTPError_JSONMessageIsOneLine(t *testing.T) {
	err := httpError(400, []byte(`{"message":"first\nsecond"}`))
	if got := err.Error(); got != "HTTP 400: first second" {
		t.Errorf("error = %q", got)
	}
}

func TestTokenCache(t *testing.T) {
	calls := 0
	fail := true
	c := &azTokenCache{fetch: func(context.Context) (string, time.Time, error) {
		calls++
		if fail {
			return "", time.Time{}, errors.New("not logged in")
		}
		return fmt.Sprintf("tok-%d", calls), time.Now().Add(time.Hour), nil
	}}
	ctx := context.Background()

	// A failure is shared by requests within the retry delay.
	_, err1 := c.Token(ctx)
	_, err2 := c.Token(ctx)
	if err1 == nil || err2 == nil || calls != 1 {
		t.Fatalf("errors %v/%v after %d calls", err1, err2, calls)
	}

	// After the delay a fresh token is fetched and then reused.
	fail = false
	c.errUntil = time.Time{}
	tok, err := c.Token(ctx)
	if err != nil || tok != "tok-2" {
		t.Fatalf("token = %q, %v", tok, err)
	}
	if again, _ := c.Token(ctx); again != tok || calls != 2 {
		t.Errorf("cached token not reused: %q after %d calls", again, calls)
	}

	// Invalidating the cached token forces a refetch once it is old enough;
	// other tokens and fresh rejections are ignored.
	c.Invalidate("other")
	c.Invalidate(tok)
	if again, _ := c.Token(ctx); again != tok {
		t.Error("invalidating a different or fresh token should keep the cache")
	}
	c.fetchedAt = time.Now().Add(-azTokenMinAge)
	c.Invalidate(tok)
	if again, _ := c.Token(ctx); again != "tok-3" {
		t.Errorf("token after invalidate = %q", again)
	}
}

func TestResolveIterationPath_FallsBackToLiteral(t *testing.T) {
	a := newTestAzure(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r) // team does not exist
	})
	ctx := context.Background()
	if got, err := a.resolveIterationPath(ctx, "Release 1"); err != nil || got != "Release 1" {
		t.Errorf("lookup failure: got %q, %v", got, err)
	}
	if got, err := a.resolveIterationPath(ctx, "proj"); err != nil || got != "proj" {
		t.Errorf("project root: got %q, %v", got, err)
	}
	if _, err := a.resolveIterationPath(ctx, "previous"); err == nil {
		t.Error("previous cannot be resolved without iterations")
	}
}

func TestAzErrorLineSkipsWarnings(t *testing.T) {
	stderr := "WARNING: A newer version of az is available\nERROR: Please run 'az login' to setup account.\n"
	if got := azErrorLine(stderr); got != "ERROR: Please run 'az login' to setup account." {
		t.Errorf("azErrorLine = %q", got)
	}
	if got := azErrorLine("something odd\nmore"); got != "something odd" {
		t.Errorf("fallback = %q", got)
	}
}

func TestAcquireGivesUpWhenCancelled(t *testing.T) {
	for range cap(sem) {
		sem <- struct{}{}
	}
	defer func() {
		for range cap(sem) {
			<-sem
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("acquire = %v", err)
	}
}

func TestTokenCacheWaitersHonourContext(t *testing.T) {
	release := make(chan struct{})
	c := &azTokenCache{fetch: func(context.Context) (string, time.Time, error) {
		<-release
		return "tok", time.Now().Add(time.Hour), nil
	}}
	defer close(release)

	go func() { _, _ = c.Token(context.Background()) }() // slow az call
	for {
		c.mu.Lock()
		busy := c.inflight != nil
		c.mu.Unlock()
		if busy {
			break
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Token(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("waiting caller got %v, want its deadline", err)
	}
}
