package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// azureDevOpsResource is the Entra ID application ID for Azure DevOps.
const azureDevOpsResource = "499b84ac-1321-427f-aa17-267ca6975798"

// azTokenMargin is how long before expiry a cached token is refreshed.
const azTokenMargin = 5 * time.Minute

// azTokenFallbackTTL bounds reuse when az does not report an expiry.
const azTokenFallbackTTL = 5 * time.Minute

// azTokenRetryDelay is how long a failed az call is reported to concurrent
// and subsequent requests before az is tried again.
const azTokenRetryDelay = 5 * time.Second

// tokenSource provides bearer tokens for Azure DevOps requests.
type tokenSource interface {
	Token(ctx context.Context) (string, error)
	// Invalidate drops token if it is still cached, after Azure rejected it.
	Invalidate(token string)
}

// azTokenCache keeps the az CLI access token in memory. Starting az takes
// roughly a second, so fetching a token per request made every refresh slow.
// The token is never written to disk (see ADR-002).
type azTokenCache struct {
	fetch func(context.Context) (string, time.Time, error)

	mu       sync.Mutex
	token    string
	expires  time.Time
	err      error // last fetch failure, reused until errUntil
	errUntil time.Time
}

var azTokens = &azTokenCache{fetch: fetchAzToken}

// Token returns a cached token, or fetches a new one from the az CLI when the
// cached token is missing or close to expiry. A failed fetch is remembered
// briefly so that a refresh's concurrent requests do not each run az in turn.
func (c *azTokenCache) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if c.token != "" && now.Before(c.expires) {
		return c.token, nil
	}
	if c.err != nil && now.Before(c.errUntil) {
		return "", c.err
	}
	token, expires, err := c.fetch(ctx)
	if err != nil {
		if ctx.Err() == nil {
			c.err, c.errUntil = err, now.Add(azTokenRetryDelay)
		}
		return "", err
	}
	c.token, c.expires = token, cacheUntil(expires, now)
	c.err = nil
	return token, nil
}

// Invalidate drops token if it is still the cached one, so the next request
// asks az again.
func (c *azTokenCache) Invalidate(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == token {
		c.token = ""
		c.expires = time.Time{}
	}
}

// cacheUntil returns when a token expiring at expires should stop being reused.
func cacheUntil(expires, now time.Time) time.Time {
	if expires.IsZero() {
		return now.Add(azTokenFallbackTTL)
	}
	return expires.Add(-azTokenMargin)
}

// fetchAzToken acquires an Azure DevOps access token via the az CLI.
func fetchAzToken(ctx context.Context) (string, time.Time, error) {
	out, err := exec.CommandContext(ctx, "az", "account", "get-access-token",
		"--resource", azureDevOpsResource, "--output", "json").Output()
	if err != nil {
		return "", time.Time{}, azCLIError(err)
	}
	return parseAzToken(out)
}

func azCLIError(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("azure-boards: az CLI not found — install Azure CLI and run 'az login'")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if msg := firstLine(string(exitErr.Stderr)); msg != "" {
			return fmt.Errorf("azure-boards: az CLI failed (run 'az login'?): %s", msg)
		}
	}
	return fmt.Errorf("azure-boards: az CLI failed (run 'az login'?): %w", err)
}

// parseAzToken extracts the access token and its expiry from az output.
// Newer az versions report expires_on as Unix seconds; older ones only report
// expiresOn as a local timestamp.
func parseAzToken(out []byte) (string, time.Time, error) {
	var result struct {
		AccessToken string          `json:"accessToken"`
		ExpiresOn   string          `json:"expiresOn"`
		ExpiresOnTS json.RawMessage `json:"expires_on"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("azure-boards: failed to parse az token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("azure-boards: az returned empty access token — run 'az login' first")
	}

	var expires time.Time
	if ts := strings.Trim(string(result.ExpiresOnTS), `"`); ts != "" {
		if secs, err := strconv.ParseInt(ts, 10, 64); err == nil {
			expires = time.Unix(secs, 0)
		}
	}
	if expires.IsZero() && result.ExpiresOn != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05.999999", result.ExpiresOn, time.Local); err == nil {
			expires = t
		}
	}
	return result.AccessToken, expires, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return truncateText(strings.TrimSpace(s), 200)
}

func truncateText(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
