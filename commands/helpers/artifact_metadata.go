package helpers

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/in-toto/in-toto-golang/in_toto"
	_ "github.com/in-toto/in-toto-golang/in_toto"
	slsa_common "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	artifactsStatementFormat  = "%v-generateStatement.json"
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
}

type generateStatementOptions struct {
	artifactName string
	files        map[string]os.FileInfo
	wd           string
	jobID        int64
}

func (g *artifactStatementGenerator) generateStatementToFile(opts generateStatementOptions) (string, error) {
	statement, err := g.generateStatement(opts)
	if err != nil {
		return "", err
	}

	file := filepath.Join(opts.wd, fmt.Sprintf(artifactsStatementFormat, opts.artifactName))

	b, err := json.MarshalIndent(statement, "", " ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(file, b, 0o644)
	return file, err
}

func (g *artifactStatementGenerator) generateStatement(opts generateStatementOptions) (*in_toto.ProvenanceStatementSLSA1, error) {

	header, err := g.generateStatementHeader(opts.files, slsa.PredicateSLSAProvenance)
	if err != nil {
		return nil, err
	}
	start, end, err := g.parseTimings()
	if err != nil {
		return nil, err
	}

	jobID := strconv.Itoa(int(opts.jobID))
	predicate := g.generatePredicate(jobID, start, end)
	return &in_toto.ProvenanceStatementSLSA1{
		StatementHeader: header,
		Predicate:       predicate,
	}, nil
}

func (g *artifactStatementGenerator) generatePredicate(jobId string, start time.Time, end time.Time) slsa.ProvenancePredicate {
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

	resolvedDeps := []slsa.ResourceDescriptor{{
		URI:    g.RepoURL,
		Digest: map[string]string{"sha256": g.RepoDigest},
	}}

	builderVersion := map[string]string{
		"gitlab-runner": g.version(),
	}

	return slsa.ProvenancePredicate{
		BuildDefinition: slsa.ProvenanceBuildDefinition{
			BuildType:            fmt.Sprintf(attestationTypeFormat, g.version()),
			ExternalParameters:   externalParams,
			InternalParameters:   internalParams,
			ResolvedDependencies: resolvedDeps,
		},
		RunDetails: slsa.ProvenanceRunDetails{
			Builder: slsa.Builder{
				ID:                  strconv.Itoa(int(g.RunnerID)),
				Version:             builderVersion,
				BuilderDependencies: nil,
			},
			BuildMetadata: slsa.BuildMetadata{
				InvocationID: jobId,
				StartedOn:    &start,
				FinishedOn:   &end,
			},
			Byproducts: nil,
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
