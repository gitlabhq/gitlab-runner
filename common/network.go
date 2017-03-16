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

type JRJobInfo struct {
	Name        string `json:"name"`
	Stage       string `json:"stage"`
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type JRGitInfoRefType string

const (
	RefTypeBranch JRGitInfoRefType = "branch"
	RefTypeTag                     = "tag"
)

type JRGitInfo struct {
	RepoURL   string           `json:"repo_url"`
	Ref       string           `json:"ref"`
	Sha       string           `json:"sha"`
	BeforeSha string           `json:"before_sha"`
	RefType   JRGitInfoRefType `json:"ref_type"`
}

type JRRunnerInfo struct {
	Timeout int `json:"timeout"`
}

type JRStepScript []string

type JRStepWhen string

const (
	StepWhenOnFailure JRStepWhen = "on_failure"
	StepWhenOnSuccess            = "on_success"
	StepWhenAlways               = "always"
)

type JRStep struct {
	Name         string       `json:"name"`
	Script       JRStepScript `json:"script"`
	Timeout      int          `json:"timeout"`
	When         JRStepWhen   `json:"when"`
	AllowFailure bool         `json:"allow_failure"`
}

type JRSteps []JRStep

type JRImage struct {
	Name string `json:"name"`
}

type JRServices []JRImage

type JRArtifactPaths []string

type JRArtifactWhen string

const (
	ArtifactWhenOnFailure JRArtifactWhen = "on_failure"
	ArtifactWhenOnSuccess                = "on_success"
	ArtifactWhenAlways                   = "always"
)

type JRArtifact struct {
	Name      string          `json:"name"`
	Untracted bool            `json:"untracted"`
	Paths     JRArtifactPaths `json:"paths"`
	When      JRArtifactWhen  `json:"when"`
	ExpireIn  string          `json:"expire_in"`
}

type JRArtifacts []JRArtifact

type JRCache struct {
	Key       string          `json:"key"`
	Untracted bool            `json:"untracted"`
	Paths     JRArtifactPaths `json:"paths"`
}

type JRCaches []JRCache

type JRCredentials struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type JRDependencyArtifactsFile struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type JRDependency struct {
	ID            int                       `json:"id"`
	Token         string                    `json:"token"`
	Name          string                    `json:"name"`
	ArtifactsFile JRDependencyArtifactsFile `json:"artifacts_file"`
}

type JRDependencies []JRDependency

type JobResponse struct {
	ID            int             `json:"id"`
	Token         string          `json:"token"`
	AllowGitFetch bool            `json:"allow_git_fetch"`
	JobInfo       JRJobInfo       `json:"job_info"`
	GitInfo       JRGitInfo       `json:"git_info"`
	RunnerInfo    JRRunnerInfo    `json:"runner_info"`
	Variables     BuildVariables  `json:"variables"` // TODO: Rename BuildVariables to JobVariables
	Steps         JRSteps         `json:"steps"`
	Image         JRImage         `json:"image"`
	Services      JRServices      `json:"services"`
	Artifacts     JRArtifacts     `json:"artifacts"`
	Cache         JRCaches        `json:"cache"`
	Credentials   []JRCredentials `json:"credentials"`
	Dependencies  JRDependencies  `json:"dependencies"`

	// TODO: Introduces changes in scripts execution!

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
