package common

import (
	"io"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/url"
)

type UpdateState int
type UploadState int
type DownloadState int
type JobState string

const (
	Pending JobState = "pending"
	Running          = "running"
	Failed           = "failed"
	Success          = "success"
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
	Variables bool `json:"variables"`
	Image     bool `json:"image"`
	Services  bool `json:"services"`
	Artifacts bool `json:"features"`
	Cache     bool `json:"cache"`
}

type RegisterRunnerRequest struct {
	Info        VersionInfo `json:"info,omitempty"`
	Token       string      `json:"token,omitempty"`
	Description string      `json:"description,omitempty"`
	Tags        string      `json:"tag_list,omitempty"`
	RunUntagged bool        `json:"run_untagged"`
	Locked      bool        `json:"locked"`
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
	Features     FeaturesInfo `json:"features"`
}

type JobRequest struct {
	Info       VersionInfo `json:"info,omitempty"`
	Token      string      `json:"token,omitempty"`
	LastUpdate string      `json:"last_update,omitempty"`
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
	RefTypeTag                   = "tag"
)

type GitInfo struct {
	RepoURL   string         `json:"repo_url"`
	Ref       string         `json:"ref"`
	Sha       string         `json:"sha"`
	BeforeSha string         `json:"before_sha"`
	RefType   GitInfoRefType `json:"ref_type"`
}

type RunnerInfo struct {
	Timeout int `json:"timeout"`
}

type StepScript []string

type StepName string

const (
	StepNameScript      StepName = "script"
	StepNameAfterScript          = "after_script"
)

type StepWhen string

const (
	StepWhenOnFailure StepWhen = "on_failure"
	StepWhenOnSuccess          = "on_success"
	StepWhenAlways             = "always"
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
	Name string `json:"name"`
}

type Services []Image

type ArtifactPaths []string

type ArtifactWhen string

const (
	ArtifactWhenOnFailure ArtifactWhen = "on_failure"
	ArtifactWhenOnSuccess              = "on_success"
	ArtifactWhenAlways                 = "always"
)

type Artifact struct {
	Name      string        `json:"name"`
	Untracked bool          `json:"untracked"`
	Paths     ArtifactPaths `json:"paths"`
	When      ArtifactWhen  `json:"when"`
	ExpireIn  string        `json:"expire_in"`
}

type Artifacts []Artifact

type Cache struct {
	Key       string        `json:"key"`
	Untracked bool          `json:"untracked"`
	Paths     ArtifactPaths `json:"paths"`
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

type JobResponse struct {
	ID            int            `json:"id"`
	Token         string         `json:"token"`
	AllowGitFetch bool           `json:"allow_git_fetch"`
	JobInfo       JobInfo        `json:"job_info"`
	GitInfo       GitInfo        `json:"git_info"`
	RunnerInfo    RunnerInfo     `json:"runner_info"`
	Variables     BuildVariables `json:"variables"` // TODO: Rename BuildVariables to JobVariables
	Steps         Steps          `json:"steps"`
	Image         Image          `json:"image"`
	Services      Services       `json:"services"`
	Artifacts     Artifacts      `json:"artifacts"`
	Cache         Caches         `json:"cache"`
	Credentials   []Credentials  `json:"credentials"`
	Dependencies  Dependencies   `json:"dependencies"`

	TLSCAChain string `json:"-"`
}

func (j *JobResponse) RepoCleanURL() string {
	return url_helpers.CleanURL(j.GitInfo.RepoURL)
}

type UpdateJobRequest struct {
	Info  VersionInfo `json:"info,omitempty"`
	Token string      `json:"token,omitempty"`
	State JobState    `json:"state,omitempty"`
	Trace *string     `json:"trace,omitempty"`
}

type JobCredentials struct {
	ID        int    `long:"id" env:"CI_BUILD_ID" description:"The build ID to upload artifacts for"`
	Token     string `long:"token" env:"CI_BUILD_TOKEN" required:"true" description:"Build token"`
	URL       string `long:"url" env:"CI_SERVER_URL" required:"true" description:"GitLab CI URL"`
	TLSCAFile string `long:"tls-ca-file" env:"CI_SERVER_TLS_CA_FILE" description:"File containing the certificates to verify the peer when using HTTPS"`
}

func (j *JobCredentials) GetURL() string {
	return j.URL
}

func (j *JobCredentials) GetTLSCAFile() string {
	return j.TLSCAFile
}

func (j *JobCredentials) GetToken() string {
	return j.Token
}

type JobTrace interface {
	io.Writer
	Success()
	Fail(err error)
	Aborted() chan interface{}
	IsStdout() bool
}

type JobTracePatch interface {
	Patch() []byte
	Offset() int
	Limit() int
	SetNewOffset(newOffset int)
	ValidateRange() bool
}

type Network interface {
	RegisterRunner(config RunnerCredentials, description, tags string, runUntagged, locked bool) *RegisterRunnerResponse
	VerifyRunner(config RunnerCredentials) bool
	UnregisterRunner(config RunnerCredentials) bool
	RequestJob(config RunnerConfig) (*JobResponse, bool)
	UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, id int, state JobState, trace *string) UpdateState
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, tracePart JobTracePatch) UpdateState
	DownloadArtifacts(config JobCredentials, artifactsFile string) DownloadState
	UploadRawArtifacts(config JobCredentials, reader io.Reader, baseName string, expireIn string) UploadState
	UploadArtifacts(config JobCredentials, artifactsFile string) UploadState
	ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) JobTrace
}
