//go:build !integration

package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	prov_v1 "github.com/in-toto/attestation/go/predicates/provenance/v1"
	ita_v1 "github.com/in-toto/attestation/go/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

	var testsStatementV1 = func(
		version string,
		g *artifactStatementGenerator,
		opts generateStatementOptions,
	) *ita_v1.Statement {
		externalParams, err := g.externalParams(g.JobName, g.RepoURL)
		require.NoError(t, err)

		internalParams, err := g.internalParams(opts.jobID)
		require.NoError(t, err)

		provenance := &prov_v1.Provenance{
			BuildDefinition: &prov_v1.BuildDefinition{
				BuildType:          fmt.Sprintf(attestationTypeFormat, version),
				ExternalParameters: externalParams,
				InternalParameters: internalParams,
				ResolvedDependencies: []*ita_v1.ResourceDescriptor{{
					Uri:    g.RepoURL,
					Digest: map[string]string{"sha256": g.RepoDigest},
				}},
			},
			RunDetails: &prov_v1.RunDetails{
				Builder: &prov_v1.Builder{
					Id: fmt.Sprintf(attestationRunnerIDFormat, g.RepoURL, g.RunnerID),
					Version: map[string]string{
						"gitlab-runner": version,
					},
				},
				Metadata: &prov_v1.BuildMetadata{
					InvocationId: fmt.Sprint(opts.jobID),
					StartedOn:    timestamppb.New(startedAt),
					FinishedOn:   timestamppb.New(endedAt),
				},
			},
		}

		predicateJSON, err := protojson.Marshal(provenance)
		require.NoError(t, err)

		predicate := &structpb.Struct{}
		err = protojson.Unmarshal(predicateJSON, predicate)
		require.NoError(t, err)

		return &ita_v1.Statement{
			Type:          in_toto.StatementInTotoV01,
			PredicateType: slsa_v1.PredicateSLSAProvenance,
			Subject: []*ita_v1.ResourceDescriptor{
				{
					Name:   tmpFile.Name(),
					Digest: map[string]string{"sha256": hex.EncodeToString(checksum)},
				},
			},
			Predicate: predicate,
		}
	}

	var testStatement = func(
		version string,
		g *artifactStatementGenerator,
		opts generateStatementOptions) any {
		switch g.SLSAProvenanceVersion {
		case slsaProvenanceVersion1:
			return testsStatementV1(version, g, opts)
		default:
			panic("unreachable, invalid statement version")
		}
	}

	var setVersion = func(version string) (string, func()) {
		originalVersion := common.AppVersion.Version
		common.AppVersion.Version = version

		return version, func() {
			common.AppVersion.Version = originalVersion
		}
	}

	var newGenerator = func(slsaVersion string) *artifactStatementGenerator {
		return &artifactStatementGenerator{
			RunnerID:              1001,
			RepoURL:               "testurl",
			RepoDigest:            "testdigest",
			JobName:               "testjobname",
			ExecutorName:          "testexecutorname",
			RunnerName:            "testrunnername",
			Parameters:            []string{"testparam"},
			StartedAtRFC3339:      startedAtRFC3339,
			EndedAtRFC3339:        endedAtRFC3339,
			SLSAProvenanceVersion: slsaVersion,
		}
	}

	tests := map[string]struct {
		opts          generateStatementOptions
		newGenerator  func(slsaVersion string) *artifactStatementGenerator
		expected      func(*artifactStatementGenerator, generateStatementOptions) (any, func())
		expectedError error
	}{
		"basic": {
			newGenerator: newGenerator,
			opts: generateStatementOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				artifactsWd:  tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactStatementGenerator, opts generateStatementOptions) (any, func()) {
				version, cleanup := setVersion("v1.0.0")
				return testStatement(version, g, opts), cleanup
			},
		},
		"basic version isn't prefixed so use REVISION": {
			newGenerator: newGenerator,
			opts: generateStatementOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				artifactsWd:  tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactStatementGenerator, opts generateStatementOptions) (any, func()) {
				return testStatement(common.AppVersion.Revision, g, opts), func() {}
			},
		},
		"files subject doesn't exist": {
			newGenerator: newGenerator,
			opts: generateStatementOptions{
				artifactName: "artifact-name",
				files: map[string]os.FileInfo{
					tmpFile.Name(): fileInfo{name: tmpFile.Name()},
					"nonexisting":  fileInfo{name: "nonexisting"},
				},
				artifactsWd: tmpDir,
				jobID:       1000,
			},
			expectedError: os.ErrNotExist,
		},
		"non-regular file": {
			newGenerator: newGenerator,
			opts: generateStatementOptions{
				artifactName: "artifact-name",
				files: map[string]os.FileInfo{
					tmpFile.Name(): fileInfo{name: tmpFile.Name()},
					"dir":          fileInfo{name: "im-a-dir", mode: fs.ModeDir}},
				artifactsWd: tmpDir,
				jobID:       1000,
			},
			expected: func(g *artifactStatementGenerator, opts generateStatementOptions) (any, func()) {
				return testStatement(common.AppVersion.Revision, g, opts), func() {}
			},
		},
		"no parameters": {
			newGenerator: func(v string) *artifactStatementGenerator {
				g := newGenerator(v)
				g.Parameters = nil

				return g
			},
			opts: generateStatementOptions{
				artifactName: "artifact-name",
				files:        map[string]os.FileInfo{tmpFile.Name(): fileInfo{name: tmpFile.Name()}},
				artifactsWd:  tmpDir,
				jobID:        1000,
			},
			expected: func(g *artifactStatementGenerator, opts generateStatementOptions) (any, func()) {
				return testStatement(common.AppVersion.Revision, g, opts), func() {}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			for _, v := range []string{slsaProvenanceVersion1} {
				t.Run(v, func(t *testing.T) {
					g := tt.newGenerator(v)

					var expected any
					if tt.expected != nil {
						var cleanup func()
						expected, cleanup = tt.expected(g, tt.opts)
						defer cleanup()
					}

					f, err := g.generateStatementToFile(tt.opts)
					if tt.expectedError == nil {
						require.NoError(t, err)
					} else {
						assert.Empty(t, f)
						assert.ErrorIs(t, err, tt.expectedError)
						return
					}

					filename := filepath.Base(f)
					assert.Equal(t, fmt.Sprintf(artifactsStatementFormat, tt.opts.artifactName), filename)

					file, err := os.Open(f)
					require.NoError(t, err)
					defer file.Close()

					b, err := io.ReadAll(file)
					require.NoError(t, err)

					indented, err := protojson.MarshalOptions{Multiline: true, Indent: " "}.Marshal(expected.(*ita_v1.Statement))
					require.NoError(t, err)

					assert.Equal(t, string(indented), string(b))
					assert.Contains(t, string(indented), startedAtRFC3339)
					assert.Contains(t, string(indented), endedAtRFC3339)
				})
			}
		})
	}
}

func TestGeneratePredicateV1(t *testing.T) {
	gen := &artifactStatementGenerator{
		RunnerID:              1001,
		RepoURL:               "testurl",
		RepoDigest:            "testdigest",
		JobName:               "testjobname",
		ExecutorName:          "testexecutorname",
		RunnerName:            "testrunnername",
		Parameters:            []string{"testparam"},
		SLSAProvenanceVersion: slsaProvenanceVersion1,
	}

	startTime := time.Now()
	endTime := startTime.Add(time.Minute)

	originalVersion := common.AppVersion.Version
	testVersion := "vTest"
	common.AppVersion.Version = testVersion

	defer func() {
		common.AppVersion.Version = originalVersion
	}()

	actualPredicate, err := gen.generateSLSAv1Predicate(10001, startTime, endTime)
	require.NoError(t, err)

	expectedBuildType := fmt.Sprintf(attestationTypeFormat, testVersion)
	assert.Equal(t, expectedBuildType, actualPredicate.BuildDefinition.BuildType)
}
