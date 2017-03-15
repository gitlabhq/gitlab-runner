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

type JobArtifacts struct {
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

type JobInfo struct {
	ID        int           `json:"id,omitempty"`
	Sha       string        `json:"sha,omitempty"`
	RefName   string        `json:"ref,omitempty"`
	Token     string        `json:"token"`
	Name      string        `json:"name"`
	Stage     string        `json:"stage"`
	Tag       bool          `json:"tag"`
	Artifacts *JobArtifacts `json:"artifacts_file"`
}

type JobResponseCredentials struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type JobResponse struct {
	ID              int            `json:"id,omitempty"`
	ProjectID       int            `json:"project_id,omitempty"`
	Commands        string         `json:"commands,omitempty"`
	RepoURL         string         `json:"repo_url,omitempty"`
	Sha             string         `json:"sha,omitempty"`
	RefName         string         `json:"ref,omitempty"`
	BeforeSha       string         `json:"before_sha,omitempty"`
	AllowGitFetch   bool           `json:"allow_git_fetch,omitempty"`
	Timeout         int            `json:"timeout,omitempty"`
	Variables       BuildVariables `json:"variables"`
	Options         BuildOptions   `json:"options"`
	Token           string         `json:"token"`
	Name            string         `json:"name"`
	Stage           string         `json:"stage"`
	Tag             bool           `json:"tag"`
	DependsOnBuilds []JobInfo      `json:"depends_on_builds"`
	TLSCAChain      string         `json:"-"`

	Credentials []JobResponseCredentials `json:"credentials,omitempty"`
}

func (b *JobResponse) RepoCleanURL() (ret string) {
	return url_helpers.CleanURL(b.RepoURL)
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

type BuildTrace interface {
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
	UpdateJob(config RunnerConfig, id int, state JobState, trace *string) UpdateState
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, tracePart JobTracePatch) UpdateState
	DownloadArtifacts(config JobCredentials, artifactsFile string) DownloadState
	UploadRawArtifacts(config JobCredentials, reader io.Reader, baseName string, expireIn string) UploadState
	UploadArtifacts(config JobCredentials, artifactsFile string) UploadState
	ProcessBuild(config RunnerConfig, buildCredentials *JobCredentials) BuildTrace
}
