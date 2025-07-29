package hosted_runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type WikiPage struct {
	Content string `json:"content"`
}

type WikiJSONTable struct {
	Fields   []WikiJSONTableField `json:"fields"`
	Items    []BridgeInfo         `json:"items"`
	Markdown bool                 `json:"markdown"`
}

type WikiJSONTableField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type GitLabWikiClient struct {
	log *slog.Logger

	token string
	url   string
}

func NewGitLabWikiClient(log *slog.Logger, baseURL string, projectID string, pageSlug string, token string) (*GitLabWikiClient, error) {
	if token == "" {
		return nil, fmt.Errorf("GitLab token is required")
	}

	return &GitLabWikiClient{
		log:   log,
		token: token,
		url:   fmt.Sprintf("%s/api/v4/projects/%s/wikis/%s", strings.TrimRight(baseURL, "/"), projectID, pageSlug),
	}, nil
}

func (c *GitLabWikiClient) Read(ctx context.Context) (WikiPage, error) {
	var v WikiPage

	c.log.Info("Reading gitlab wiki page", "url", c.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return v, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpDo(req)
	if err != nil {
		return v, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	c.log.Info("Decoding response")

	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return v, fmt.Errorf("decoding response: %w", err)
	}

	c.log.Debug("Current content", "content", v.Content)

	return v, nil
}

func (c *GitLabWikiClient) httpDo(req *http.Request) (*http.Response, error) {
	req.Header.Set("Private-Token", c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	return resp, nil
}

func (c *GitLabWikiClient) Update(ctx context.Context, page WikiPage) error {
	c.log.Debug("New content", "content", page.Content)

	c.log.Info("Encoding request")

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(page)
	if err != nil {
		return fmt.Errorf("encoding wiki page: %w", err)
	}

	c.log.Info("Updating gitlab wiki page", "url", c.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.url, buf)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpDo(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(os.Stderr, resp.Body)

		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
