//go:build !integration

package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type fileInfo struct {
	name string
	mode fs.FileMode
}

func (fi fileInfo) Name() string {
	return fi.name
}

func (fi fileInfo) Size() int64 {
	return 0
}

func (fi fileInfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi fileInfo) ModTime() time.Time {
	return time.Now()
}

func (fi fileInfo) IsDir() bool {
	return fi.mode.IsDir()
}

func (fi fileInfo) Sys() any {
	return nil
}

func TestGenerateMetadataToFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "")
	require.NoError(t, err)

	_, err = tmpFile.WriteString("testdata")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	sha := sha256.New()
	sha.Write([]byte("testdata"))
	checksum := sha.Sum(nil)

	// First format the time to RFC3339 and then parse it to get the correct precision
	startedAtRFC3339 := time.Now().Format(time.RFC3339)
	startedAt, err := time.Parse(time.RFC3339, startedAtRFC3339)
	require.NoError(t, err)

	endedAtRFC3339 := time.Now().Add(time.Minute).Format(time.RFC3339)
	endedAt, err := time.Parse(time.RFC3339, endedAtRFC3339)
	require.NoError(t, err)

	var testMetadata = func(
		version string,
		g *artifactMetadataGenerator,
		opts generateMetadataOptions,
	) AttestationMetadata {
		return AttestationMetadata{
			Type: attestationType,
			Subject: []AttestationSubject{
				{
					Name: tmpFile.Name(),
					Digest: AttestationDigest{
						Sha256: hex.EncodeToString(checksum),
					},
				},
			},
			PredicateType: attestationPredicateType,
			Predicate: AttestationPredicate{
				BuildType: fmt.Sprintf(attestationTypeFormat, version),
				Builder: AttestationPredicateBuilder{
					ID: fmt.Sprintf(attestationRunnerIDFormat, g.RepoURL, g.RunnerID),
				},
				Invocation: AttestationPredicateInvocation{
					ConfigSource: AttestationPredicateInvocationConfigSource{
						URI: g.RepoURL,
						Digest: AttestationDigest{
							Sha256: g.RepoDigest,
						},
						EntryPoint: g.JobName,
					},
					Environment: AttestationPredicateInvocationEnvironment{
						Name:         g.RunnerName,
						Executor:     g.ExecutorName,
						Architecture: common.AppVersion.Architecture,
						Job:          AttestationPredicateInvocationEnvironmentJob{ID: opts.jobID},
					},
					Parameters: AttestationPredicateInvocationParameters{
						"testparam": "",
					},
				},
			},
			Materials: make([]interface{}, 0),
			Metadata: AttestationMetadataInfo{
				BuildStartedOn: TimeRFC3339{
					Time: startedAt,
				},
				BuildFinishedOn: TimeRFC3339{
					Time: endedAt,
				},
				Reproducible: false,
				Completeness: AttestationMetadataInfoCompleteness{
					Parameters:  true,
					Environment: true,
					Materials:   false,
				},
			},
		}
	}

	var setVersion = func(version string) (string, func()) {
		originalVersion := common.AppVersion.Version
		common.AppVersion.Version = version

		return version, func() {
			common.AppVersion.Version = originalVersion
		}
	}

	var newGenerator = func() *artifactMetadataGenerator {
		return &artifactMetadataGenerator{
			RunnerID:         1001,
			RepoURL:          "testurl",
			RepoDigest:       "testdigest",
			JobName:          "testjobname",
			ExecutorName:     "testexecutorname",
			RunnerName:       "testrunnername",
			Parameters:       []string{"testparam"},
			StartedAtRFC3339: startedAtRFC3339,
			EndedAtRFC3339:   endedAtRFC3339,
		}
	}

	tests := map[string]struct {
		opts          generateMetadataOptions
		newGenerator  func() *artifactMetadataGenerator
		expected      func(*artifactMetadataGenerator, generateMetadataOptions) (AttestationMetadata, func())
		expectedError error
	}{
		"basic": {
			newGenerator: newGenerator,
			opts: generateMetadataOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				wd:           tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactMetadataGenerator, opts generateMetadataOptions) (AttestationMetadata, func()) {
				version, cleanup := setVersion("v1.0.0")
				m := testMetadata(version, g, opts)
				return m, cleanup
			},
		},
		"basic version isn't prefixed so use REVISION": {
			newGenerator: newGenerator,
			opts: generateMetadataOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				wd:           tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactMetadataGenerator, opts generateMetadataOptions) (AttestationMetadata, func()) {
				m := testMetadata(common.AppVersion.Revision, g, opts)
				return m, func() {}
			},
		},
		"files subject doesn't exist": {
			newGenerator: newGenerator,
			opts: generateMetadataOptions{
				artifactName: "artifact-name",
				files: map[string]os.FileInfo{
					tmpFile.Name(): fileInfo{name: tmpFile.Name()},
					"nonexisting":  fileInfo{name: "nonexisting"},
				},
				wd:    tmpDir,
				jobID: 1000,
			},
			expectedError: os.ErrNotExist,
		},
		"non-regular file": {
			newGenerator: newGenerator,
			opts: generateMetadataOptions{
				artifactName: "artifact-name",
				files: map[string]os.FileInfo{
					tmpFile.Name(): fileInfo{name: tmpFile.Name()},
					"dir":          fileInfo{name: "im-a-dir", mode: fs.ModeDir}},
				wd:    tmpDir,
				jobID: 1000,
			},
			expected: func(g *artifactMetadataGenerator, opts generateMetadataOptions) (AttestationMetadata, func()) {
				m := testMetadata(common.AppVersion.Revision, g, opts)
				return m, func() {}
			},
		},
		"no parameters": {
			newGenerator: func() *artifactMetadataGenerator {
				g := newGenerator()
				g.Parameters = nil

				return g
			},
			opts: generateMetadataOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				wd:           tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactMetadataGenerator, opts generateMetadataOptions) (AttestationMetadata, func()) {
				m := testMetadata(common.AppVersion.Revision, g, opts)
				m.Predicate.Invocation.Parameters = map[string]string{}
				return m, func() {}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			g := tt.newGenerator()

			var expected AttestationMetadata
			if tt.expected != nil {
				var cleanup func()
				expected, cleanup = tt.expected(g, tt.opts)
				defer cleanup()
			}

			f, err := g.generateMetadataToFile(tt.opts)
			if tt.expectedError == nil {
				require.NoError(t, err)
			} else {
				assert.Empty(t, f)
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}

			filename := filepath.Base(f)
			assert.Equal(t, fmt.Sprintf(artifactsMetadataFormat, tt.opts.artifactName), filename)

			file, err := os.Open(f)
			require.NoError(t, err)
			defer file.Close()

			b, err := io.ReadAll(file)
			require.NoError(t, err)

			var actual AttestationMetadata
			err = json.Unmarshal(b, &actual)
			require.NoError(t, err)

			require.Equal(t, expected, actual)

			// apart from being correct, make sure that the data is also properly formatted
			indented, err := json.MarshalIndent(expected, "", " ")
			require.NoError(t, err)

			assert.Equal(t, indented, b)
			assert.Contains(t, string(indented), startedAtRFC3339)
			assert.Contains(t, string(indented), endedAtRFC3339)
		})
	}
}
