//go:build integration

package helpers

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

func TestArchiveUploadExpandArgs(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	os.Setenv("expand", "expanded")
	defer os.Unsetenv("expand")

	cmd := &ArtifactsUploaderCommand{
		Name: "artifact $expand",
		JobCredentials: common.JobCredentials{
			ID:    12345,
			Token: "token",
			URL:   srv.URL,
		},
	}
	cmd.Paths = []string{"unexpanded", "path/${expand}/${expand:1:3}"}
	cmd.Exclude = []string{"unexpanded", "path/$expand/${foo:-bar}"}

	cmd.Execute(&cli.Context{})

	assert.Equal(t, "artifact expanded", cmd.Name)
	assert.Equal(t, []string{"unexpanded", "path/expanded/xpa"}, cmd.Paths)
	assert.Equal(t, []string{"unexpanded", "path/expanded/bar"}, cmd.Exclude)
}

func TestArchiveUploadRedirect(t *testing.T) {
	finalRequestReceived := false

	finalServer := httptest.NewServer(
		assertRequestPathAndMethod(t, "final", finalServerHandler(t, &finalRequestReceived)),
	)
	defer finalServer.Close()

	redirectingServer := httptest.NewServer(
		assertRequestPathAndMethod(t, "redirection", redirectingServerHandler(finalServer.URL)),
	)
	defer redirectingServer.Close()

	cmd := &ArtifactsUploaderCommand{
		JobCredentials: common.JobCredentials{
			ID:    12345,
			Token: "token",
			URL:   redirectingServer.URL,
		},
		Name:             "artifacts",
		Format:           common.ArtifactFormatZip,
		CompressionLevel: "fastest",
		network:          network.NewGitLabClient(),
		fileArchiver: fileArchiver{
			Paths: []string{
				filepath.Join(".", "testdata", "test-artifacts"),
			},
		},
	}

	defer helpers.MakeFatalToPanic()()

	assert.NotPanics(t, func() {
		cmd.Execute(&cli.Context{})
	}, "expected command not to log fatal")

	assert.True(t, finalRequestReceived)
}

func assertRequestPathAndMethod(t *testing.T, handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		assert.Equal(t, "/api/v4/jobs/12345/artifacts", r.URL.Path, "server handler: %s", handlerName)
		assert.NotEqual(t, "/api/v4/jobs/12345/jobs/12345/artifacts", r.URL.Path, "server handler: %s", handlerName)

		handler(rw, r)
	}
}

func redirectingServerHandler(finalServerURL string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Location", fmt.Sprintf("%s%s", finalServerURL, r.RequestURI))
		rw.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func finalServerHandler(t *testing.T, finalRequestReceived *bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		dir := t.TempDir()

		receiveFile(t, r, dir)

		err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			fileName := info.Name()
			fileContentBytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			assert.Equal(t, fileName, strings.TrimSpace(string(fileContentBytes)))

			return nil
		})

		assert.NoError(t, err)

		*finalRequestReceived = true
		rw.WriteHeader(http.StatusCreated)
	}
}

func receiveFile(t *testing.T, r *http.Request, targetDir string) {
	err := r.ParseMultipartForm(1024)
	require.NoError(t, err)

	formFiles := r.MultipartForm.File["file"]
	require.Len(t, formFiles, 1)

	formFile := formFiles[0]

	assert.Equal(t, "artifacts.zip", formFile.Filename)

	f, err := formFile.Open()
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	extractor, err := fastzip.NewExtractor(f, formFile.Size, targetDir)
	require.NoError(t, err)

	err = extractor.Extract(context.Background())
	require.NoError(t, err)
}
