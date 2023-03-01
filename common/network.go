package common

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
)

type UpdateState int
type PatchState int
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
	ScriptFailure       JobFailureReason = "script_failure"
	RunnerSystemFailure JobFailureReason = "runner_system_failure"
	JobExecutionTimeout JobFailureReason = "job_execution_timeout"
	UnknownFailure      JobFailureReason = "unknown_failure"
	// JobCanceled is only internal to runner, and not used inside of rails.
	JobCanceled JobFailureReason = "job_canceled"
)

const (
	UpdateSucceeded UpdateState = iota
	UpdateAcceptedButNotCompleted
	UpdateTraceValidationFailed
	UpdateNotFound
	UpdateAbort
	UpdateFailed
)

const (
	PatchSucceeded PatchState = iota
	PatchNotFound
	PatchAbort
	PatchRangeMismatch
	PatchFailed
)

const (
	UploadSucceeded UploadState = iota
	UploadTooLarge
	UploadForbidden
	UploadFailed
	UploadServiceUnavailable
	UploadRedirected
)

const (
	DownloadSucceeded DownloadState = iota
	DownloadForbidden
	DownloadUnauthorized
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
	RawVariables            bool `json:"raw_variables"`
	ArtifactsExclude        bool `json:"artifacts_exclude"`
	MultiBuildSteps         bool `json:"multi_build_steps"`
	TraceReset              bool `json:"trace_reset"`
	TraceChecksum           bool `json:"trace_checksum"`
	TraceSize               bool `json:"trace_size"`
	VaultSecrets            bool `json:"vault_secrets"`
	Cancelable              bool `json:"cancelable"`
	ReturnExitCode          bool `json:"return_exit_code"`
	ServiceVariables        bool `json:"service_variables"`
	ServiceMultipleAliases  bool `json:"service_multiple_aliases"`
}

type ConfigInfo struct {
	Gpus string `json:"gpus"`
}

type RegisterRunnerParameters struct {
	Description     string `json:"description,omitempty"`
	MaintenanceNote string `json:"maintenance_note,omitempty"`
	Tags            string `json:"tag_list,omitempty"`
	RunUntagged     bool   `json:"run_untagged"`
	Locked          bool   `json:"locked"`
	AccessLevel     string `json:"access_level,omitempty"`
	MaximumTimeout  int    `json:"maximum_timeout,omitempty"`
	Paused          bool   `json:"paused"`
}

type RegisterRunnerRequest struct {
	RegisterRunnerParameters
	Info  VersionInfo `json:"info,omitempty"`
	Token string      `json:"token,omitempty"`
}

type RegisterRunnerResponse struct {
	ID             int64     `json:"id,omitempty"`
	Token          string    `json:"token,omitempty"`
	TokenExpiresAt time.Time `json:"token_expires_at,omitempty"`
}

type VerifyRunnerRequest struct {
	Token    string `json:"token,omitempty"`
	SystemID string `json:"system_id,omitempty"`
}

type VerifyRunnerResponse struct {
	ID             int64     `json:"id,omitempty"`
	Token          string    `json:"token,omitempty"`
	TokenExpiresAt time.Time `json:"token_expires_at,omitempty"`
}

type UnregisterRunnerRequest struct {
	Token string `json:"token,omitempty"`
}

type ResetTokenRequest struct {
	Token string `json:"token,omitempty"`
}

type ResetTokenResponse struct {
	Token           string `json:"token,omitempty"`
	TokenObtainedAt time.Time
	TokenExpiresAt  time.Time `json:"token_expires_at,omitempty"`
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
	Config       ConfigInfo   `json:"config,omitempty"`
}

type JobRequest struct {
	Info       VersionInfo  `json:"info,omitempty"`
	Token      string       `json:"token,omitempty"`
	SystemID   string       `json:"system_id,omitempty"`
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
	ProjectID   int64  `json:"project_id"`
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
	Name         string             `json:"name"`
	Alias        string             `json:"alias,omitempty"`
	Command      []string           `json:"command,omitempty"`
	Entrypoint   []string           `json:"entrypoint,omitempty"`
	Ports        []Port             `json:"ports,omitempty"`
	Variables    JobVariables       `json:"variables,omitempty"`
	PullPolicies []DockerPullPolicy `json:"pull_policy,omitempty"`
}

func (i *Image) Aliases() []string { return strings.Fields(strings.ReplaceAll(i.Alias, ",", " ")) }

type Port struct {
	Number   int    `json:"number,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
}

type Services []Image

type ArtifactPaths []string

type ArtifactExclude []string

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
	Name      string          `json:"name"`
	Untracked bool            `json:"untracked"`
	Paths     ArtifactPaths   `json:"paths"`
	Exclude   ArtifactExclude `json:"exclude"`
	When      ArtifactWhen    `json:"when"`
	Type      string          `json:"artifact_type"`
	Format    ArtifactFormat  `json:"artifact_format"`
	ExpireIn  string          `json:"expire_in"`
}

type Artifacts []Artifact

type Cache struct {
	Key       string        `json:"key"`
	Untracked bool          `json:"untracked"`
	Policy    CachePolicy   `json:"policy"`
	Paths     ArtifactPaths `json:"paths"`
	When      CacheWhen     `json:"when"`
}

type CacheWhen string

const (
	CacheWhenOnFailure CacheWhen = "on_failure"
	CacheWhenOnSuccess CacheWhen = "on_success"
	CacheWhenAlways    CacheWhen = "always"
)

func (when CacheWhen) ShouldCache(jobSuccess bool) bool {
	if jobSuccess {
		return when.OnSuccess()
	}

	return when.OnFailure()
}

func (when CacheWhen) OnSuccess() bool {
	return when == "" || when == CacheWhenOnSuccess || when == CacheWhenAlways
}

func (when CacheWhen) OnFailure() bool {
	return when == CacheWhenOnFailure || when == CacheWhenAlways
}

func (c Cache) CheckPolicy(wanted CachePolicy) (bool, error) {
	switch c.Policy {
	case CachePolicyUndefined, CachePolicyPullPush:
		return true, nil
	case CachePolicyPull, CachePolicyPush:
		return wanted == c.Policy, nil
	}

	return false, fmt.Errorf("unknown cache policy %s", c.Policy)
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
	ID            int64                   `json:"id"`
	Token         string                  `json:"token"`
	Name          string                  `json:"name"`
	ArtifactsFile DependencyArtifactsFile `json:"artifacts_file"`
}

type Dependencies []Dependency

type GitlabFeatures struct {
	TraceSections     bool               `json:"trace_sections"`
	TokenMaskPrefixes []string           `json:"token_mask_prefixes"`
	FailureReasons    []JobFailureReason `json:"failure_reasons"`
}

type Hooks []Hook

type Hook struct {
	Name   HookName   `json:"name"`
	Script StepScript `json:"script"`
}

type HookName string

const (
	HookPreGetSourcesScript  HookName = "pre_get_sources_script"
	HookPostGetSourcesScript HookName = "post_get_sources_script"
)

func (hooks Hooks) Get(name HookName) Hook {
	for _, hook := range hooks {
		if hook.Name == name {
			return hook
		}
	}

	return Hook{}
}

type JobResponse struct {
	ID            int64          `json:"id"`
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
	Secrets       Secrets        `json:"secrets,omitempty"`
	Hooks         Hooks          `json:"hooks,omitempty"`

	TLSCAChain  string `json:"-"`
	TLSAuthCert string `json:"-"`
	TLSAuthKey  string `json:"-"`
}

type Secrets map[string]Secret

type Secret struct {
	Vault *VaultSecret `json:"vault,omitempty"`

	File *bool `json:"file,omitempty"`
}

type VaultSecret struct {
	Server VaultServer `json:"server"`
	Engine VaultEngine `json:"engine"`
	Path   string      `json:"path"`
	Field  string      `json:"field"`
}

type VaultServer struct {
	URL       string    `json:"url"`
	Auth      VaultAuth `json:"auth"`
	Namespace string    `json:"namespace"`
}

type VaultAuth struct {
	Name string        `json:"name"`
	Path string        `json:"path"`
	Data VaultAuthData `json:"data"`
}

type VaultAuthData map[string]interface{}

type VaultEngine struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s Secrets) expandVariables(vars JobVariables) {
	for _, secret := range s {
		secret.expandVariables(vars)
	}
}

func (s Secret) expandVariables(vars JobVariables) {
	if s.Vault != nil {
		s.Vault.expandVariables(vars)
	}
}

// IsFile defines whether the variable should be of type FILE or no.
//
// The default behavior is to represent the variable as FILE type.
// If defined by the user - set to whatever was chosen.
func (s Secret) IsFile() bool {
	if s.File == nil {
		return SecretVariableDefaultsToFile
	}

	return *s.File
}

func (s *VaultSecret) expandVariables(vars JobVariables) {
	s.Server.expandVariables(vars)
	s.Engine.expandVariables(vars)

	s.Path = vars.ExpandValue(s.Path)
	s.Field = vars.ExpandValue(s.Field)
}

func (s *VaultSecret) AuthName() string {
	return s.Server.Auth.Name
}

func (s *VaultSecret) AuthPath() string {
	return s.Server.Auth.Path
}

func (s *VaultSecret) AuthData() auth_methods.Data {
	return auth_methods.Data(s.Server.Auth.Data)
}

func (s *VaultSecret) EngineName() string {
	return s.Engine.Name
}

func (s *VaultSecret) EnginePath() string {
	return s.Engine.Path
}

func (s *VaultSecret) SecretPath() string {
	return s.Path
}

func (s *VaultSecret) SecretField() string {
	return s.Field
}

func (s *VaultServer) expandVariables(vars JobVariables) {
	s.URL = vars.ExpandValue(s.URL)
	s.Namespace = vars.ExpandValue(s.Namespace)

	s.Auth.expandVariables(vars)
}

func (a *VaultAuth) expandVariables(vars JobVariables) {
	a.Name = vars.ExpandValue(a.Name)
	a.Path = vars.ExpandValue(a.Path)

	for field, value := range a.Data {
		a.Data[field] = vars.ExpandValue(fmt.Sprintf("%s", value))
	}
}

func (e *VaultEngine) expandVariables(vars JobVariables) {
	e.Name = vars.ExpandValue(e.Name)
	e.Path = vars.ExpandValue(e.Path)
}

func (j *JobResponse) RepoCleanURL() string {
	return url_helpers.CleanURL(j.GitInfo.RepoURL)
}

func (j *JobResponse) JobURL() string {
	url := strings.TrimSuffix(j.RepoCleanURL(), ".git")

	return fmt.Sprintf("%s/-/jobs/%d", url, j.ID)
}

type UpdateJobRequest struct {
	Info          VersionInfo      `json:"info,omitempty"`
	Token         string           `json:"token,omitempty"`
	State         JobState         `json:"state,omitempty"`
	FailureReason JobFailureReason `json:"failure_reason,omitempty"`
	Checksum      string           `json:"checksum,omitempty"` // deprecated
	Output        JobTraceOutput   `json:"output,omitempty"`
	ExitCode      int              `json:"exit_code,omitempty"`
}

type JobTraceOutput struct {
	Checksum string `json:"checksum,omitempty"`
	Bytesize int    `json:"bytesize,omitempty"`
}

//nolint:lll
type JobCredentials struct {
	ID          int64  `long:"id" env:"CI_JOB_ID" description:"The build ID to download and upload artifacts for"`
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
	ID            int64
	State         JobState
	FailureReason JobFailureReason
	Output        JobTraceOutput
	ExitCode      int
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

//go:generate mockery --name=JobTrace --inpackage
type JobTrace interface {
	io.Writer
	Success()
	Fail(err error, failureData JobFailureData)
	SetCancelFunc(cancelFunc context.CancelFunc)
	Cancel() bool
	SetAbortFunc(abortFunc context.CancelFunc)
	Abort() bool
	SetFailuresCollector(fc FailuresCollector)
	SetMasked(maskOptions MaskOptions)
	SetDebugModeEnabled(isEnabled bool)
	IsStdout() bool
}

type UpdateJobResult struct {
	State             UpdateState
	CancelRequested   bool
	NewUpdateInterval time.Duration
}

type PatchTraceResult struct {
	SentOffset        int
	CancelRequested   bool
	State             PatchState
	NewUpdateInterval time.Duration
}

func NewPatchTraceResult(sentOffset int, state PatchState, newUpdateInterval int) PatchTraceResult {
	return PatchTraceResult{
		SentOffset:        sentOffset,
		State:             state,
		NewUpdateInterval: time.Duration(newUpdateInterval) * time.Second,
	}
}

//go:generate mockery --name=Network --inpackage
type Network interface {
	RegisterRunner(config RunnerCredentials, parameters RegisterRunnerParameters) *RegisterRunnerResponse
	VerifyRunner(config RunnerCredentials, systemID string) *VerifyRunnerResponse
	UnregisterRunner(config RunnerCredentials) bool
	ResetToken(runner RunnerCredentials, systemID string) *ResetTokenResponse
	ResetTokenWithPAT(runner RunnerCredentials, systemID string, pat string) *ResetTokenResponse
	RequestJob(ctx context.Context, config RunnerConfig, sessionInfo *SessionInfo) (*JobResponse, bool)
	UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, content []byte,
		startOffset int, debugModeEnabled bool) PatchTraceResult
	DownloadArtifacts(config JobCredentials, artifactsFile io.WriteCloser, directDownload *bool) DownloadState
	UploadRawArtifacts(config JobCredentials, reader io.ReadCloser, options ArtifactsOptions) (UploadState, string)
	ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) (JobTrace, error)
}
