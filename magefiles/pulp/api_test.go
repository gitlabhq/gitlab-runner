//go:build !integration

package pulp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	prcPath  = "/pulp/api/v3/content/deb/package_release_components/"
	archPath = "/pulp/api/v3/content/deb/release_architectures/"
)

func TestAPIClientCreateContent(t *testing.T) {
	var taskPolls, flakyPolls atomic.Int32

	// The handler runs on the server goroutine, so it uses assert: require would call FailNow off the test goroutine.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		assert.Equal(t, "runner", user)
		assert.Equal(t, "secret", pass)

		switch r.URL.Path {
		case prcPath:
			if r.Method == http.MethodGet {
				assert.Equal(t, "1", r.URL.Query().Get("limit"))
				results := []map[string]string{}
				if r.URL.Query().Get("package") == "/pkg/existing/" {
					results = append(results, map[string]string{"pulp_href": "/prc/existing/"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"count": len(results), "results": results})
				return
			}
			var body map[string]string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "/pkg/1/", body["package"])
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"pulp_href": "/prc/1/"})
		case archPath:
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"task": "/pulp/api/v3/tasks/t1/"})
		case "/pulp/api/v3/tasks/t1/":
			state := "running"
			if taskPolls.Add(1) >= 2 {
				state = "completed"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"state": state, "created_resources": []string{"/arch/1/"}})
		case "/pulp/api/v3/content/deb/flaky/":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"task": "/pulp/api/v3/tasks/flaky/"})
		case "/pulp/api/v3/tasks/flaky/":
			// first two polls fail at the gateway, the task is still reported as completed afterwards
			if flakyPolls.Add(1) <= 2 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "completed", "created_resources": []string{"/arch/flaky/"}})
		case "/pulp/api/v3/content/deb/multi/":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"task": "/pulp/api/v3/tasks/multi/"})
		case "/pulp/api/v3/tasks/multi/":
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "completed", "created_resources": []string{"/arch/1/", "/repo/version/1/"}})
		case "/pulp/api/v3/content/deb/failing/":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"task": "/pulp/api/v3/tasks/t2/"})
		case "/pulp/api/v3/tasks/t2/":
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "failed", "error": map[string]string{"description": "nope"}})
		case "/pulp/api/v3/content/deb/forbidden/":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"detail": "no"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newAPIClient(srv.URL+"/", "runner", "secret")
	c.taskPollInterval = time.Millisecond
	ctx := t.Context()

	t.Run("201 returns the unit", func(t *testing.T) {
		href, err := c.createContent(ctx, prcPath, map[string]string{"package": "/pkg/1/", "release_component": "/rc/1/"})
		require.NoError(t, err)
		require.Equal(t, "/prc/1/", href)
	})

	t.Run("202 waits for the task", func(t *testing.T) {
		href, err := c.createContent(ctx, archPath, map[string]string{"distribution": "jammy", "architecture": "amd64"})
		require.NoError(t, err)
		require.Equal(t, "/arch/1/", href)
		require.GreaterOrEqual(t, taskPolls.Load(), int32(2))
	})

	t.Run("failed polls are retried until the deadline", func(t *testing.T) {
		href, err := c.createContent(ctx, "/pulp/api/v3/content/deb/flaky/", map[string]string{})
		require.NoError(t, err)
		require.Equal(t, "/arch/flaky/", href)
		require.Equal(t, int32(3), flakyPolls.Load())
	})

	t.Run("task with more than one created resource is an error", func(t *testing.T) {
		_, err := c.createContent(ctx, "/pulp/api/v3/content/deb/multi/", map[string]string{})
		require.ErrorContains(t, err, "2 created resources, want exactly 1")
	})

	t.Run("find returns the existing unit", func(t *testing.T) {
		href, err := c.findContent(ctx, prcPath, map[string]string{"package": "/pkg/existing/", "release_component": "/rc/1/"})
		require.NoError(t, err)
		require.Equal(t, "/prc/existing/", href)
	})

	t.Run("find returns empty when there is no unit", func(t *testing.T) {
		href, err := c.findContent(ctx, prcPath, map[string]string{"package": "/pkg/new/", "release_component": "/rc/1/"})
		require.NoError(t, err)
		require.Empty(t, href)
	})

	t.Run("failed task", func(t *testing.T) {
		_, err := c.createContent(ctx, "/pulp/api/v3/content/deb/failing/", map[string]string{})
		require.ErrorContains(t, err, "failed")
		require.ErrorContains(t, err, "nope")
	})

	t.Run("http error", func(t *testing.T) {
		_, err := c.createContent(ctx, "/pulp/api/v3/content/deb/forbidden/", map[string]string{})
		require.ErrorContains(t, err, "unexpected status 403")
	})
}

func TestAPIClientTaskTimeout(t *testing.T) {
	newServer := func(pollStatus int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]string{"task": "/pulp/api/v3/tasks/slow/"})
				return
			}
			w.WriteHeader(pollStatus)
			_ = json.NewEncoder(w).Encode(map[string]any{"state": "running"})
		}))
	}

	newClient := func(srv *httptest.Server) *apiClient {
		c := newAPIClient(srv.URL, "u", "p")
		c.taskPollInterval = time.Millisecond
		c.taskTimeout = 20 * time.Millisecond
		return c
	}

	t.Run("task never completes", func(t *testing.T) {
		srv := newServer(http.StatusOK)
		defer srv.Close()

		_, err := newClient(srv).createContent(t.Context(), "/x/", map[string]string{})
		require.ErrorContains(t, err, "deadline exceeded")
	})

	t.Run("polls keep failing", func(t *testing.T) {
		srv := newServer(http.StatusBadGateway)
		defer srv.Close()

		_, err := newClient(srv).createContent(t.Context(), "/x/", map[string]string{})
		require.ErrorContains(t, err, "deadline exceeded")
		require.ErrorContains(t, err, "unexpected status 502")
	})
}

func TestNewAPIClientFromCLIConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)
	dir := filepath.Join(configDir, "pulp")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	configFile := filepath.Join(dir, "cli.toml")

	t.Run("missing file", func(t *testing.T) {
		_, err := newAPIClientFromCLIConfig()
		require.ErrorContains(t, err, "reading pulp cli config")
	})

	t.Run("incomplete config", func(t *testing.T) {
		require.NoError(t, os.WriteFile(configFile, []byte("[cli]\nbase_url = \"https://pulp.example/\"\nusername = \"runner\"\n"), 0o600))
		_, err := newAPIClientFromCLIConfig()
		require.ErrorContains(t, err, "missing base_url, username or password")
	})

	t.Run("valid config", func(t *testing.T) {
		require.NoError(t, os.WriteFile(configFile, []byte("[cli]\nbase_url = \"https://pulp.example/\"\nusername = \"runner\"\npassword = \"s3cret\"\nformat = \"json\"\n"), 0o600))
		c, err := newAPIClientFromCLIConfig()
		require.NoError(t, err)
		require.Equal(t, "https://pulp.example", c.baseURL)
		require.Equal(t, "runner", c.username)
		require.Equal(t, "s3cret", c.password)
	})
}
