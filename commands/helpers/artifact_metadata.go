package helpers

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/in-toto/in-toto-golang/in_toto"
	slsa_common "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa_v02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	artifactsStatementFormat  = "%v-statement.json"
	attestationTypeFormat     = "https://gitlab.com/gitlab-org/gitlab-runner/-/blob/%v/PROVENANCE.md"
	attestationRunnerIDFormat = "%v/-/runners/%v"
)

type artifactStatementGenerator struct {
	GenerateArtifactsMetadata bool     `long:"generate-artifacts-metadata"`
	RunnerID                  int64    `long:"runner-id"`
	RepoURL                   string   `long:"repo-url"`
	RepoDigest                string   `long:"repo-digest"`
	JobName                   string   `long:"job-name"`
	ExecutorName              string   `long:"executor-name"`
	RunnerName                string   `long:"runner-name"`
	Parameters                []string `long:"metadata-parameter"`
	StartedAtRFC3339          string   `long:"started-at"`
	EndedAtRFC3339            string   `long:"ended-at"`
	SLSAProvenanceVersion     string   `env:"SLSA_PROVENANCE_SCHEMA_VERSION"`
}

type generateStatementOptions struct {
	artifactName string
	files        map[string]os.FileInfo
	wd           string
	jobID        int64
}

var provenanceSchemas = map[string]struct{}{
	slsaProvenanceVersion02: {},
	slsaProvenanceVersion1:  {},
}

const (
	slsaProvenanceVersion1  string = "v1"
	slsaProvenanceVersion02 string = "v0.2"
)

func (g *artifactStatementGenerator) generateStatementToFile(opts generateStatementOptions) (string, error) {
	var statement interface{}

	start, end, err := g.parseTimings()
	if err != nil {
		return "", err
	}

	jobID := strconv.Itoa(int(opts.jobID))

	// check if provided slsa provenance version is one that is supported
	_, ok := provenanceSchemas[g.SLSAProvenanceVersion]
	if !ok {
		g.SLSAProvenanceVersion = slsaProvenanceVersion02
	}

	switch g.SLSAProvenanceVersion {
	case slsaProvenanceVersion02:
		header, headerErr := g.generateStatementHeader(opts.files, slsa_v02.PredicateSLSAProvenance)
		if headerErr != nil {
			return "", headerErr
		}
		var err error
		statement, err = g.generateSLSAv02Statement(header, jobID, &start, &end)
		if err != nil {
			return "", err
		}
	case slsaProvenanceVersion1:
		header, headerErr := g.generateStatementHeader(opts.files, slsa_v02.PredicateSLSAProvenance)
		if headerErr != nil {
			return "", err
		}
		var err error
		statement, err = g.generateSLSAv1Statement(header, jobID, &start, &end)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown slsa provenance version: %s", g.SLSAProvenanceVersion)
	}

	b, err := json.MarshalIndent(statement, "", " ")
	if err != nil {
		return "", err
	}

	file := filepath.Join(opts.wd, fmt.Sprintf(artifactsStatementFormat, opts.artifactName))

	err = os.WriteFile(file, b, 0o644)
	return file, err
}

func (g *artifactStatementGenerator) generateSLSAv1Statement(header in_toto.StatementHeader, jobID string, start *time.Time, end *time.Time) (*in_toto.ProvenanceStatementSLSA1, error) {
	predicate := g.generateSLSAv1Predicate(jobID, start, end)
	return &in_toto.ProvenanceStatementSLSA1{
		StatementHeader: header,
		Predicate:       predicate,
	}, nil
}

func (g *artifactStatementGenerator) generateSLSAv1Predicate(jobId string, start *time.Time, end *time.Time) slsa_v1.ProvenancePredicate {
	externalParams := make(map[string]string, len(g.Parameters))
	for _, param := range g.Parameters {
		externalParams[param] = ""
	}

	externalParams["entryPoint"] = g.JobName
	externalParams["source"] = g.RepoURL

	internalParams := map[string]string{
		"name":         g.RunnerName,
		"executor":     g.ExecutorName,
		"architecture": common.AppVersion.Architecture,
		"job":          jobId,
	}

	resolvedDeps := []slsa_v1.ResourceDescriptor{{
		URI:    g.RepoURL,
		Digest: map[string]string{"sha256": g.RepoDigest},
	}}

	builderVersion := map[string]string{
		"gitlab-runner": g.version(),
	}

	return slsa_v1.ProvenancePredicate{
		BuildDefinition: slsa_v1.ProvenanceBuildDefinition{
			BuildType:            fmt.Sprintf(attestationTypeFormat, g.version()),
			ExternalParameters:   externalParams,
			InternalParameters:   internalParams,
			ResolvedDependencies: resolvedDeps,
		},
		RunDetails: slsa_v1.ProvenanceRunDetails{
			Builder: slsa_v1.Builder{
				ID:                  fmt.Sprintf(attestationRunnerIDFormat, g.RepoURL, g.RunnerID),
				Version:             builderVersion,
				BuilderDependencies: nil,
			},
			BuildMetadata: slsa_v1.BuildMetadata{
				InvocationID: jobId,
				StartedOn:    start,
				FinishedOn:   end,
			},
			Byproducts: nil,
		},
	}
}

func (g *artifactStatementGenerator) generateSLSAv02Statement(header in_toto.StatementHeader, jobID string, start *time.Time, end *time.Time) (*in_toto.ProvenanceStatementSLSA02, error) {
	predicate := g.generateSLSAv02Predicate(jobID, start, end)
	return &in_toto.ProvenanceStatementSLSA02{
		StatementHeader: header,
		Predicate:       predicate,
	}, nil
}

func (g *artifactStatementGenerator) generateSLSAv02Predicate(jobID string, start *time.Time, end *time.Time) slsa_v02.ProvenancePredicate {
	params := make(map[string]string, len(g.Parameters))
	for _, param := range g.Parameters {
		params[param] = ""
	}

	type EnvironmentJob struct {
		ID string `json:"id"`
	}

	type Environment struct {
		Name         string         `json:"name"`
		Executor     string         `json:"executor"`
		Architecture string         `json:"architecture"`
		Job          EnvironmentJob `json:"job"`
	}

	return slsa_v02.ProvenancePredicate{
		Builder:   slsa_common.ProvenanceBuilder{ID: fmt.Sprintf(attestationRunnerIDFormat, g.RepoURL, g.RunnerID)},
		BuildType: fmt.Sprintf(attestationTypeFormat, g.version()),
		Invocation: slsa_v02.ProvenanceInvocation{
			ConfigSource: slsa_v02.ConfigSource{
				URI: g.RepoURL,
				Digest: slsa_common.DigestSet{
					"sha256": g.RepoDigest,
				},
			},
			Parameters: params,
			Environment: Environment{
				Name:         g.RunnerName,
				Executor:     g.ExecutorName,
				Architecture: common.AppVersion.Architecture,
				Job:          EnvironmentJob{ID: jobID},
			},
		},
		Metadata: &slsa_v02.ProvenanceMetadata{
			BuildStartedOn:  start,
			BuildFinishedOn: end,
			Reproducible:    false,
			Completeness: slsa_v02.ProvenanceComplete{
				Parameters:  true,
				Environment: true,
				Materials:   false,
			},
		},
	}
}

func (g *artifactStatementGenerator) generateStatementHeader(artifacts map[string]os.FileInfo, predicateType string) (in_toto.StatementHeader, error) {
	subjects, err := g.generateSubjects(artifacts)
	if err != nil {
		return in_toto.StatementHeader{}, err
	}
	return in_toto.StatementHeader{
		Type:          in_toto.StatementInTotoV01,
		PredicateType: predicateType,
		Subject:       subjects,
	}, nil
}

func (g *artifactStatementGenerator) version() string {
	if strings.HasPrefix(common.AppVersion.Version, "v") {
		return common.AppVersion.Version
	}

	return common.AppVersion.Revision
}

func (g *artifactStatementGenerator) parseTimings() (time.Time, time.Time, error) {
	startedAt, err := time.Parse(time.RFC3339, g.StartedAtRFC3339)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	endedAt, err := time.Parse(time.RFC3339, g.EndedAtRFC3339)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return startedAt, endedAt, nil
}

func (g *artifactStatementGenerator) generateSubjects(files map[string]os.FileInfo) ([]in_toto.Subject, error) {
	subjects := make([]in_toto.Subject, 0, len(files))

	h := sha256.New()
	br := bufio.NewReader(nil)
	subjectGeneratorFunc := func(file string) (in_toto.Subject, error) {
		f, err := os.Open(file)
		if err != nil {
			return in_toto.Subject{}, err
		}
		defer f.Close()

		br.Reset(f)
		h.Reset()
		if _, err := io.Copy(h, br); err != nil {
			return in_toto.Subject{}, err
		}

		digestSet := make(slsa_common.DigestSet, 1)
		digestSet["sha256"] = hex.EncodeToString(h.Sum(nil))

		return in_toto.Subject{
			Name:   file,
			Digest: digestSet,
		}, nil
	}

	for file, fi := range files {
		if !fi.Mode().IsRegular() {
			continue
		}

		subject, err := subjectGeneratorFunc(file)
		if err != nil {
			return nil, err
		}

		subjects = append(subjects, subject)
	}

	return subjects, nil
}
