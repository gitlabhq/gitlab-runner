package pulp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// apiClient talks to the Pulp REST API directly for the deb content types that pulp-cli-deb
// (as of 0.5.0) cannot create: release_architecture and package_release_component.
type apiClient struct {
	baseURL  string
	username string
	password string
	http     *http.Client

	taskPollInterval time.Duration
	taskTimeout      time.Duration
}

func newAPIClient(baseURL, username, password string) *apiClient {
	return &apiClient{
		baseURL:          strings.TrimRight(baseURL, "/"),
		username:         username,
		password:         password,
		http:             &http.Client{Timeout: 60 * time.Second},
		taskPollInterval: time.Second,
		taskTimeout:      10 * time.Minute,
	}
}

type cliConfig struct {
	CLI struct {
		BaseURL  string `toml:"base_url"`
		Username string `toml:"username"`
		Password string `toml:"password"`
	} `toml:"cli"`
}

// newAPIClientFromCLIConfig uses the config written by `pulp config create`, so the REST calls use
// exactly what the pulp CLI uses. click and os.UserConfigDir agree on the directory on Linux and macOS.
func newAPIClientFromCLIConfig() (*apiClient, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "pulp", "cli.toml")

	var cfg cliConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("reading pulp cli config %s: %w", path, err)
	}
	if cfg.CLI.BaseURL == "" || cfg.CLI.Username == "" || cfg.CLI.Password == "" {
		return nil, fmt.Errorf("pulp cli config %s is missing base_url, username or password", path)
	}
	return newAPIClient(cfg.CLI.BaseURL, cfg.CLI.Username, cfg.CLI.Password), nil
}

type taskResponse struct {
	Task string `json:"task"`
}

type task struct {
	State            string   `json:"state"`
	Error            any      `json:"error"`
	CreatedResources []string `json:"created_resources"`
}

type contentUnit struct {
	PulpHref string `json:"pulp_href"`
}

type contentList struct {
	Results []contentUnit `json:"results"`
}

// findContent returns the href of the first unit matching the query, or "" if there is none.
func (c *apiClient) findContent(ctx context.Context, path string, query map[string]string) (string, error) {
	q := url.Values{"limit": {"1"}}
	for k, v := range query {
		q.Set(k, v)
	}

	status, body, err := c.do(ctx, http.MethodGet, path+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GET %s: unexpected status %d: %s", path, status, body)
	}

	var list contentList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("decoding response from %s: %w", path, err)
	}
	if len(list.Results) == 0 {
		return "", nil
	}
	return list.Results[0].PulpHref, nil
}

// createContent posts to a content endpoint and returns the href of the created (or retrieved)
// unit. Pulp answers either with 201 and the unit itself, or with 202 and a task whose
// created_resources holds the unit once it completes.
func (c *apiClient) createContent(ctx context.Context, path string, body any) (string, error) {
	slog.Info("creating pulp content", "path", path, "body", body)

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	status, respBody, err := c.do(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	switch status {
	case http.StatusCreated:
		var unit contentUnit
		if err := json.Unmarshal(respBody, &unit); err != nil {
			return "", fmt.Errorf("decoding response from %s: %w", path, err)
		}
		if unit.PulpHref == "" {
			return "", fmt.Errorf("response from %s has no pulp_href", path)
		}
		return unit.PulpHref, nil
	case http.StatusAccepted:
		var tr taskResponse
		if err := json.Unmarshal(respBody, &tr); err != nil {
			return "", fmt.Errorf("decoding task response from %s: %w", path, err)
		}
		return c.waitForCreatedResource(ctx, tr.Task)
	default:
		return "", fmt.Errorf("POST %s: unexpected status %d: %s", path, status, respBody)
	}
}

func (c *apiClient) waitForCreatedResource(ctx context.Context, taskHref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.taskTimeout)
	defer cancel()

	ticker := time.NewTicker(c.taskPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		href, done, err := c.pollTask(ctx, taskHref)
		if done {
			return href, err
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("polling pulp task failed, retrying", "task", taskHref, "error", err)
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("waiting for task %s: %w", taskHref, errors.Join(ctx.Err(), lastErr))
		case <-ticker.C:
		}
	}
}

// pollTask reads the task once. done is false while the task is still running or the poll itself
// failed; a failed poll is reported as err but is not final, the caller keeps polling until its deadline.
func (c *apiClient) pollTask(ctx context.Context, taskHref string) (href string, done bool, err error) {
	status, body, err := c.do(ctx, http.MethodGet, taskHref, nil)
	if err != nil {
		return "", false, err
	}
	if status != http.StatusOK {
		return "", false, fmt.Errorf("GET %s: unexpected status %d: %s", taskHref, status, body)
	}

	var t task
	if err := json.Unmarshal(body, &t); err != nil {
		return "", false, fmt.Errorf("decoding task %s: %w", taskHref, err)
	}

	switch t.State {
	case "completed":
		if len(t.CreatedResources) != 1 {
			return "", true, fmt.Errorf("task %s completed with %d created resources, want exactly 1: %v", taskHref, len(t.CreatedResources), t.CreatedResources)
		}
		return t.CreatedResources[0], true, nil
	case "failed", "canceled":
		return "", true, fmt.Errorf("task %s %s: %v", taskHref, t.State, t.Error)
	default:
		return "", false, nil
	}
}

func (c *apiClient) do(ctx context.Context, method, path string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("reading response of %s %s: %w", method, path, err)
	}
	return resp.StatusCode, respBody, nil
}
