package helpers

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	prov_v1 "github.com/in-toto/attestation/go/predicates/provenance/v1"
	ita_v1 "github.com/in-toto/attestation/go/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	slsa_v1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	artifactsWd  string
	jobID        int64
}

const (
	slsaProvenanceVersion1       = "v1"
	defaultSLSAProvenanceVersion = slsaProvenanceVersion1
)

func (g *artifactStatementGenerator) generateStatementToFile(opts generateStatementOptions) (string, error) {
	start, end, err := g.parseTimings()
	if err != nil {
		return "", err
	}

	if g.SLSAProvenanceVersion != slsaProvenanceVersion1 {
		logrus.Warnf("Unknown SLSA provenance version %s, defaulting to %s", g.SLSAProvenanceVersion, defaultSLSAProvenanceVersion)
	}

	subjects, err := g.generateSubjects(opts.files)
	if err != nil {
		return "", err
	}

	provenance, err := g.generateSLSAv1Predicate(opts.jobID, start, end)
	if err != nil {
		return "", err
	}

	predicateJSON, err := protojson.Marshal(provenance)
	if err != nil {
		return "", err
	}

	predicate := &structpb.Struct{}
	if err := protojson.Unmarshal(predicateJSON, predicate); err != nil {
		return "", err
	}

	statement := &ita_v1.Statement{
		Type:          in_toto.StatementInTotoV01,
		PredicateType: slsa_v1.PredicateSLSAProvenance,
		Subject:       subjects,
		Predicate:     predicate,
	}

	b, err := protojson.MarshalOptions{Multiline: true, Indent: " "}.Marshal(statement)
	if err != nil {
		return "", err
	}

	file := filepath.Join(opts.artifactsWd, fmt.Sprintf(artifactsStatementFormat, opts.artifactName))

	err = os.WriteFile(file, b, 0o644)
	return file, err
}

func (g *artifactStatementGenerator) generateSLSAv1Predicate(jobId int64, start time.Time, end time.Time) (*prov_v1.Provenance, error) {
	externalParams, err := g.externalParams(g.JobName, g.RepoURL)
	if err != nil {
		return nil, err
	}

	internalParams, err := g.internalParams(jobId)
	if err != nil {
		return nil, err
	}

	return &prov_v1.Provenance{
		BuildDefinition: &prov_v1.BuildDefinition{
			BuildType:          fmt.Sprintf(attestationTypeFormat, g.version()),
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
					"gitlab-runner": g.version(),
				},
			},
			Metadata: &prov_v1.BuildMetadata{
				InvocationId: fmt.Sprint(jobId),
				StartedOn:    timestamppb.New(start),
				FinishedOn:   timestamppb.New(end),
			},
		},
	}, nil
}

func (g *artifactStatementGenerator) externalParams(jobName, repoURL string) (*structpb.Struct, error) {
	paramsMap := make(map[string]any, len(g.Parameters))
	for _, param := range g.Parameters {
		paramsMap[param] = ""
	}

	paramsMap["entryPoint"] = jobName
	paramsMap["source"] = repoURL

	params, err := structpb.NewStruct(paramsMap)
	if err != nil {
		return nil, err
	}

	return params, nil
}

func (g *artifactStatementGenerator) internalParams(jobId int64) (*structpb.Struct, error) {
	return structpb.NewStruct(map[string]any{
		"name":         g.RunnerName,
		"executor":     g.ExecutorName,
		"architecture": common.AppVersion.Architecture,
		"job":          strconv.FormatInt(jobId, 10),
	})
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

func (g *artifactStatementGenerator) generateSubjects(files map[string]os.FileInfo) ([]*ita_v1.ResourceDescriptor, error) {
	subjects := make([]*ita_v1.ResourceDescriptor, 0, len(files))

	h := sha256.New()
	br := bufio.NewReader(nil)
	subjectGeneratorFunc := func(file string) (*ita_v1.ResourceDescriptor, error) {
		f, err := os.Open(file)
		if err != nil {
			return &ita_v1.ResourceDescriptor{}, err
		}
		defer f.Close()

		br.Reset(f)
		h.Reset()
		if _, err := io.Copy(h, br); err != nil {
			return &ita_v1.ResourceDescriptor{}, err
		}

		return &ita_v1.ResourceDescriptor{
			Name:   file,
			Digest: map[string]string{"sha256": hex.EncodeToString(h.Sum(nil))},
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
