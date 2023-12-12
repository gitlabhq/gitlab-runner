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
	"strings"
	"time"

	"github.com/in-toto/in-toto-golang/in_toto"
	slsa_common "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa_v02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	artifactsStatementFormat  = "%v-metadata.json"
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
	SLSAProvenanceVersion     string   `long:"schema-version"`
}

type generateStatementOptions struct {
	artifactName string
	files        map[string]os.FileInfo
	wd           string
	jobID        int64
}

const (
	slsaProvenanceVersion1       = "v1"
	slsaProvenanceVersion02      = "v0.2"
	defaultSLSAProvenanceVersion = slsaProvenanceVersion02
)

var provenanceSchemaPredicateType = map[string]string{
	slsaProvenanceVersion1:  slsa_v1.PredicateSLSAProvenance,
	slsaProvenanceVersion02: slsa_v02.PredicateSLSAProvenance,
}

func (g *artifactStatementGenerator) generateStatementToFile(opts generateStatementOptions) (string, error) {
	start, end, err := g.parseTimings()
	if err != nil {
		return "", err
	}

	provenanceVersion := g.SLSAProvenanceVersion
	if provenanceVersion != slsaProvenanceVersion1 && provenanceVersion != slsaProvenanceVersion02 {
		logrus.Warnln(fmt.Sprintf("Unknown SLSA provenance version %s, defaulting to %s", provenanceVersion, defaultSLSAProvenanceVersion))
		provenanceVersion = defaultSLSAProvenanceVersion
	}

	header, err := g.generateStatementHeader(opts.files, provenanceSchemaPredicateType[provenanceVersion])
	if err != nil {
		return "", err
	}

	var statement any
	switch provenanceVersion {
	case slsaProvenanceVersion1:
		statement = &in_toto.ProvenanceStatementSLSA1{
			StatementHeader: header,
			Predicate:       g.generateSLSAv1Predicate(opts.jobID, start, end),
		}
	case slsaProvenanceVersion02:
		statement = &in_toto.ProvenanceStatementSLSA02{
			StatementHeader: header,
			Predicate:       g.generateSLSAv02Predicate(opts.jobID, start, end),
		}
	}

	b, err := json.MarshalIndent(statement, "", " ")
	if err != nil {
		return "", err
	}

	file := filepath.Join(opts.wd, fmt.Sprintf(artifactsStatementFormat, opts.artifactName))

	err = os.WriteFile(file, b, 0o644)
	return file, err
}

func (g *artifactStatementGenerator) generateSLSAv1Predicate(jobId int64, start time.Time, end time.Time) slsa_v1.ProvenancePredicate {
	externalParams := g.params()
	externalParams["entryPoint"] = g.JobName
	externalParams["source"] = g.RepoURL

	return slsa_v1.ProvenancePredicate{
		BuildDefinition: slsa_v1.ProvenanceBuildDefinition{
			BuildType:          fmt.Sprintf(attestationTypeFormat, g.version()),
			ExternalParameters: externalParams,
			InternalParameters: map[string]string{
				"name":         g.RunnerName,
				"executor":     g.ExecutorName,
				"architecture": common.AppVersion.Architecture,
				"job":          fmt.Sprint(jobId),
			},
			ResolvedDependencies: []slsa_v1.ResourceDescriptor{{
				URI:    g.RepoURL,
				Digest: map[string]string{"sha256": g.RepoDigest},
			}},
		},
		RunDetails: slsa_v1.ProvenanceRunDetails{
			Builder: slsa_v1.Builder{
				ID: fmt.Sprintf(attestationRunnerIDFormat, g.RepoURL, g.RunnerID),
				Version: map[string]string{
					"gitlab-runner": g.version(),
				},
			},
			BuildMetadata: slsa_v1.BuildMetadata{
				InvocationID: fmt.Sprint(jobId),
				StartedOn:    &start,
				FinishedOn:   &end,
			},
		},
	}
}

type slsaV02Environment struct {
	Name         string                `json:"name"`
	Executor     string                `json:"executor"`
	Architecture string                `json:"architecture"`
	Job          slsaV02EnvironmentJob `json:"job"`
}

type slsaV02EnvironmentJob struct {
	ID int64 `json:"id"`
}

func (g *artifactStatementGenerator) generateSLSAv02Predicate(jobID int64, start time.Time, end time.Time) slsa_v02.ProvenancePredicate {
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
			Parameters: g.params(),
			Environment: slsaV02Environment{
				Name:         g.RunnerName,
				Executor:     g.ExecutorName,
				Architecture: common.AppVersion.Architecture,
				Job:          slsaV02EnvironmentJob{ID: jobID},
			},
		},
		Metadata: &slsa_v02.ProvenanceMetadata{
			BuildStartedOn:  &start,
			BuildFinishedOn: &end,
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

func (g *artifactStatementGenerator) params() map[string]string {
	params := make(map[string]string, len(g.Parameters))
	for _, param := range g.Parameters {
		params[param] = ""
	}

	return params
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
