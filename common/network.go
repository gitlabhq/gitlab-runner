package common

import (
	"context"
	"fmt"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

type UpdateState int
type UploadState int
type DownloadState int
type JobState string
type JobFailureReason string

const (
	Pending JobState = "pending"
	Running JobState = "running"
	Failed  JobState = "failed"
	Success JobState = "success"
)

const (
	NoneFailure         JobFailureReason = ""
	ScriptFailure       JobFailureReason = "script_failure"
	RunnerSystemFailure JobFailureReason = "runner_system_failure"
	JobExecutionTimeout JobFailureReason = "job_execution_timeout"
)

const (
	UpdateSucceeded UpdateState = iota
	UpdateNotFound
	UpdateAbort
	UpdateFailed
	UpdateRangeMismatch
)

const (
	UploadSucceeded UploadState = iota
	UploadTooLarge
	UploadForbidden
	UploadFailed
)

const (
	DownloadSucceeded DownloadState = iota
	DownloadForbidden
	DownloadFailed
	DownloadNotFound
)

type FeaturesInfo struct {
	Variables               bool `json:"variables"`
	Image                   bool `json:"image"`
	Services                bool `json:"services"`
	Artifacts               bool `json:"artifacts"`
	Cache                   bool `json:"cache"`
	Shared                  bool `json:"shared"`
	UploadMultipleArtifacts bool `json:"upload_multiple_artifacts"`
	UploadRawArtifacts      bool `json:"upload_raw_artifacts"`
	Session                 bool `json:"session"`
	Terminal                bool `json:"terminal"`
	Refspecs                bool `json:"refspecs"`
	Masking                 bool `json:"masking"`
	Proxy                   bool `json:"proxy"`
}

type RegisterRunnerParameters struct {
	Description    string `json:"description,omitempty"`
	Tags           string `json:"tag_list,omitempty"`
	RunUntagged    bool   `json:"run_untagged"`
	Locked         bool   `json:"locked"`
	AccessLevel    string `json:"access_level,omitempty"`
	MaximumTimeout int    `json:"maximum_timeout,omitempty"`
	Active         bool   `json:"active"`
}

type RegisterRunnerRequest struct {
	RegisterRunnerParameters
	Info  VersionInfo `json:"info,omitempty"`
	Token string      `json:"token,omitempty"`
}

type RegisterRunnerResponse struct {
	Token string `json:"token,omitempty"`
}

type VerifyRunnerRequest struct {
	Token string `json:"token,omitempty"`
}

type UnregisterRunnerRequest struct {
	Token string `json:"token,omitempty"`
}

type VersionInfo struct {
	Name         string       `json:"name,omitempty"`
	Version      string       `json:"version,omitempty"`
	Revision     string       `json:"revision,omitempty"`
	Platform     string       `json:"platform,omitempty"`
	Architecture string       `json:"architecture,omitempty"`
	Executor     string       `json:"executor,omitempty"`
	Shell        string       `json:"shell,omitempty"`
	Features     FeaturesInfo `json:"features"`
}

type JobRequest struct {
	Info       VersionInfo  `json:"info,omitempty"`
	Token      string       `json:"token,omitempty"`
	LastUpdate string       `json:"last_update,omitempty"`
	Session    *SessionInfo `json:"session,omitempty"`
}

type SessionInfo struct {
	URL           string `json:"url,omitempty"`
	Certificate   string `json:"certificate,omitempty"`
	Authorization string `json:"authorization,omitempty"`
}

type JobInfo struct {
	Name        string `json:"name"`
	Stage       string `json:"stage"`
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type GitInfoRefType string

const (
	RefTypeBranch GitInfoRefType = "branch"
	RefTypeTag    GitInfoRefType = "tag"
)

type GitInfo struct {
	RepoURL   string         `json:"repo_url"`
	Ref       string         `json:"ref"`
	Sha       string         `json:"sha"`
	BeforeSha string         `json:"before_sha"`
	RefType   GitInfoRefType `json:"ref_type"`
	Refspecs  []string       `json:"refspecs"`
	Depth     int            `json:"depth"`
}

type RunnerInfo struct {
	Timeout int `json:"timeout"`
}

type StepScript []string

type StepName string

const (
	StepNameScript      StepName = "script"
	StepNameAfterScript StepName = "after_script"
)

type StepWhen string

const (
	StepWhenOnFailure StepWhen = "on_failure"
	StepWhenOnSuccess StepWhen = "on_success"
	StepWhenAlways    StepWhen = "always"
)

type CachePolicy string

const (
	CachePolicyUndefined CachePolicy = ""
	CachePolicyPullPush  CachePolicy = "pull-push"
	CachePolicyPull      CachePolicy = "pull"
	CachePolicyPush      CachePolicy = "push"
)

type Step struct {
	Name         StepName   `json:"name"`
	Script       StepScript `json:"script"`
	Timeout      int        `json:"timeout"`
	When         StepWhen   `json:"when"`
	AllowFailure bool       `json:"allow_failure"`
}

type Steps []Step

type Image struct {
	Name       string   `json:"name"`
	Alias      string   `json:"alias,omitempty"`
	Command    []string `json:"command,omitempty"`
	Entrypoint []string `json:"entrypoint,omitempty"`
	Ports      []Port   `json:"ports,omitempty"`
}

type Port struct {
	Number   int    `json:"number,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
}

type Services []Image

type ArtifactPaths []string

type ArtifactWhen string

const (
	ArtifactWhenOnFailure ArtifactWhen = "on_failure"
	ArtifactWhenOnSuccess ArtifactWhen = "on_success"
	ArtifactWhenAlways    ArtifactWhen = "always"
)

func (when ArtifactWhen) OnSuccess() bool {
	return when == "" || when == ArtifactWhenOnSuccess || when == ArtifactWhenAlways
}

func (when ArtifactWhen) OnFailure() bool {
	return when == ArtifactWhenOnFailure || when == ArtifactWhenAlways
}

type ArtifactFormat string

const (
	ArtifactFormatDefault ArtifactFormat = ""
	ArtifactFormatZip     ArtifactFormat = "zip"
	ArtifactFormatGzip    ArtifactFormat = "gzip"
	ArtifactFormatRaw     ArtifactFormat = "raw"
)

type Artifact struct {
	Name      string         `json:"name"`
	Untracked bool           `json:"untracked"`
	Paths     ArtifactPaths  `json:"paths"`
	When      ArtifactWhen   `json:"when"`
	Type      string         `json:"artifact_type"`
	Format    ArtifactFormat `json:"artifact_format"`
	ExpireIn  string         `json:"expire_in"`
}

type Artifacts []Artifact

type Cache struct {
	Key       string        `json:"key"`
	Untracked bool          `json:"untracked"`
	Policy    CachePolicy   `json:"policy"`
	Paths     ArtifactPaths `json:"paths"`
}

func (c Cache) CheckPolicy(wanted CachePolicy) (bool, error) {
	switch c.Policy {
	case CachePolicyUndefined, CachePolicyPullPush:
		return true, nil
	case CachePolicyPull, CachePolicyPush:
		return wanted == c.Policy, nil
	}

	return false, fmt.Errorf("Unknown cache policy %s", c.Policy)
}

type Caches []Cache

type Credentials struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type DependencyArtifactsFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type Dependency struct {
	ID            int                     `json:"id"`
	Token         string                  `json:"token"`
	Name          string                  `json:"name"`
	ArtifactsFile DependencyArtifactsFile `json:"artifacts_file"`
}

type Dependencies []Dependency

type GitlabFeatures struct {
	TraceSections bool `json:"trace_sections"`
}

type JobResponse struct {
	ID            int            `json:"id"`
	Token         string         `json:"token"`
	AllowGitFetch bool           `json:"allow_git_fetch"`
	JobInfo       JobInfo        `json:"job_info"`
	GitInfo       GitInfo        `json:"git_info"`
	RunnerInfo    RunnerInfo     `json:"runner_info"`
	Variables     JobVariables   `json:"variables"`
	Steps         Steps          `json:"steps"`
	Image         Image          `json:"image"`
	Services      Services       `json:"services"`
	Artifacts     Artifacts      `json:"artifacts"`
	Cache         Caches         `json:"cache"`
	Credentials   []Credentials  `json:"credentials"`
	Dependencies  Dependencies   `json:"dependencies"`
	Features      GitlabFeatures `json:"features"`

	TLSCAChain  string `json:"-"`
	TLSAuthCert string `json:"-"`
	TLSAuthKey  string `json:"-"`
}

func (j *JobResponse) RepoCleanURL() string {
	return url_helpers.CleanURL(j.GitInfo.RepoURL)
}

type UpdateJobRequest struct {
	Info          VersionInfo      `json:"info,omitempty"`
	Token         string           `json:"token,omitempty"`
	State         JobState         `json:"state,omitempty"`
	FailureReason JobFailureReason `json:"failure_reason,omitempty"`
}

type JobCredentials struct {
	ID          int    `long:"id" env:"CI_JOB_ID" description:"The build ID to upload artifacts for"`
	Token       string `long:"token" env:"CI_JOB_TOKEN" required:"true" description:"Build token"`
	URL         string `long:"url" env:"CI_SERVER_URL" required:"true" description:"GitLab CI URL"`
	TLSCAFile   string `long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
	TLSCertFile string `long:"tls-cert-file" env:"CI_SERVER_TLS_CERT_FILE" description:"File containing certificate for TLS client auth with runner when using HTTPS"`
	TLSKeyFile  string `long:"tls-key-file" env:"CI_SERVER_TLS_KEY_FILE" description:"File containing private key for TLS client auth with runner when using HTTPS"`
}

func (j *JobCredentials) GetURL() string {
	return j.URL
}

func (j *JobCredentials) GetTLSCAFile() string {
	return j.TLSCAFile
}

func (j *JobCredentials) GetTLSCertFile() string {
	return j.TLSCertFile
}

func (j *JobCredentials) GetTLSKeyFile() string {
	return j.TLSKeyFile
}

func (j *JobCredentials) GetToken() string {
	return j.Token
}

type UpdateJobInfo struct {
	ID            int
	State         JobState
	FailureReason JobFailureReason
}

type ArtifactsOptions struct {
	BaseName string
	ExpireIn string
	Format   ArtifactFormat
	Type     string
}

type FailuresCollector interface {
	RecordFailure(reason JobFailureReason, runnerDescription string)
}

type JobTrace interface {
	io.Writer
	Success()
	Fail(err error, failureReason JobFailureReason)
	SetCancelFunc(cancelFunc context.CancelFunc)
	SetFailuresCollector(fc FailuresCollector)
	SetMasked(values []string)
	IsStdout() bool
}

type Network interface {
	RegisterRunner(config RunnerCredentials, parameters RegisterRunnerParameters) *RegisterRunnerResponse
	VerifyRunner(config RunnerCredentials) bool
	UnregisterRunner(config RunnerCredentials) bool
	RequestJob(config RunnerConfig, sessionInfo *SessionInfo) (*JobResponse, bool)
	UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateState
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, content []byte, startOffset int) (int, UpdateState)
	DownloadArtifacts(config JobCredentials, artifactsFile string) DownloadState
	UploadRawArtifacts(config JobCredentials, reader io.Reader, options ArtifactsOptions) UploadState
	ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) (JobTrace, error)
}
