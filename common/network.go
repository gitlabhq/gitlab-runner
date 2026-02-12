package common

import (
	"bytes"
	"context"
	"io"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

type (
	UpdateState   int
	PatchState    int
	UploadState   int
	DownloadState int
	JobState      string
)

// ContentProvider interface that can provide both the reader and optionally the content length
type ContentProvider interface {
	// GetReader returns a new io.ReadCloser for the content.
	// The caller is responsible for closing the returned ReadCloser when done.
	// Each call to GetReader must return a fresh reader starting from the beginning of the content.
	GetReader() (io.ReadCloser, error)

	// GetContentLength returns the content length and whether it's known.
	// If the second return value is false, the content length is unknown
	// and chunked transfer encoding should be used.
	GetContentLength() (int64, bool)
}

// BytesProvider implements ContentProvider for fixed, in-memory byte slices
type BytesProvider struct {
	Data []byte
}

// GetReader returns a new reader for the byte slice.
// Caller must close the returned ReadCloser when done.
func (p BytesProvider) GetReader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(p.Data)), nil
}

// GetContentLength returns the exact length of the byte slice.
func (p BytesProvider) GetContentLength() (int64, bool) {
	return int64(len(p.Data)), true // Length is known
}

// StreamProvider implements ContentProvider for streamed data where you don't want to
// or can't determine the size upfront.
type StreamProvider struct {
	// ReaderFactory should return a fresh io.ReadCloser each time it's called.
	// Each io.ReadCloser should start reading from the beginning of the content.
	ReaderFactory func() (io.ReadCloser, error)
}

// GetReader returns a new ReadCloser by calling the ReaderFactory.
// Caller must close the returned ReadCloser when done.
func (p StreamProvider) GetReader() (io.ReadCloser, error) {
	return p.ReaderFactory()
}

// GetContentLength indicates the content length is unknown.
func (p StreamProvider) GetContentLength() (int64, bool) {
	return 0, false // Length is unknown, use chunked encoding
}

const (
	Pending JobState = "pending"
	Running JobState = "running"
	Failed  JobState = "failed"
	Success JobState = "success"
)

const (
	ScriptFailure       spec.JobFailureReason = "script_failure"
	RunnerSystemFailure spec.JobFailureReason = "runner_system_failure"
	JobExecutionTimeout spec.JobFailureReason = "job_execution_timeout"
	ImagePullFailure    spec.JobFailureReason = "image_pull_failure"
	UnknownFailure      spec.JobFailureReason = "unknown_failure"

	// ConfigurationError indicates an error in the CI configuration that can only be determined by runner (and not by
	// Rails). The typical example incompatible pull policies. Since this failure reason does not exist in rails, we map
	// it to ScriptFailure below, which is more or less correct in that it's ultimately a user error.
	ConfigurationError spec.JobFailureReason = "configuration_error"

	// When defining new job failure reasons, consider if its meaning is
	// extracted from the scope of already existing one. If yes - update
	// the failureReasonsCompatibilityMap variable below.

	// Always update the allFailureReasons list

	// JobCanceled is only internal to runner, and not used inside of rails.
	JobCanceled spec.JobFailureReason = "job_canceled"
)

var (
	// allFailureReasons contains the list of all failure reasons known to runner.
	allFailureReasons = []spec.JobFailureReason{
		ScriptFailure,
		RunnerSystemFailure,
		JobExecutionTimeout,
		ImagePullFailure,
		UnknownFailure,
		ConfigurationError,
		JobCanceled,
	}

	// failureReasonsCompatibilityMap maps failure reasons that are not
	// supported by GitLab to failure reasons that are supported. This is
	// used to provide backward compatibility when new failure reasons are
	// introduced in runner but not yet supported by GitLab (and not in the
	// supported list check).
	failureReasonsCompatibilityMap = map[spec.JobFailureReason]spec.JobFailureReason{
		ImagePullFailure:   RunnerSystemFailure,
		ConfigurationError: ScriptFailure,
	}

	// A small list of failure reasons that are supported by all
	// GitLab instances.
	alwaysSupportedFailureReasons = []spec.JobFailureReason{
		ScriptFailure,
		RunnerSystemFailure,
		JobExecutionTimeout,
	}
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
	FallbackCacheKeys       bool `json:"fallback_cache_keys"`
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
	ImageExecutorOpts       bool `json:"image_executor_opts"`
	ServiceExecutorOpts     bool `json:"service_executor_opts"`
	CancelGracefully        bool `json:"cancel_gracefully"`
	NativeStepsIntegration  bool `json:"native_steps_integration"`
	TwoPhaseJobCommit       bool `json:"two_phase_job_commit"`
	JobInputs               bool `json:"job_inputs"`
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
	Info  Info   `json:"info,omitempty"`
	Token string `json:"token,omitempty"`
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

type UnregisterRunnerManagerRequest struct {
	Token    string `json:"token,omitempty"`
	SystemID string `json:"system_id"`
}

type ResetTokenRequest struct {
	Token string `json:"token,omitempty"`
}

type ResetTokenResponse struct {
	Token           string `json:"token,omitempty"`
	TokenObtainedAt time.Time
	TokenExpiresAt  time.Time `json:"token_expires_at,omitempty"`
}

type Info struct {
	Name         string       `json:"name,omitempty"`
	Version      string       `json:"version,omitempty"`
	Revision     string       `json:"revision,omitempty"`
	Platform     string       `json:"platform,omitempty"`
	Architecture string       `json:"architecture,omitempty"`
	Executor     string       `json:"executor,omitempty"`
	Shell        string       `json:"shell,omitempty"`
	Features     FeaturesInfo `json:"features"`
	Config       ConfigInfo   `json:"config,omitempty"`
	Labels       Labels       `json:"labels,omitempty"`
}

type JobRequest struct {
	Info       Info         `json:"info,omitempty"`
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

type UpdateJobRequest struct {
	Info          Info                  `json:"info,omitempty"`
	Token         string                `json:"token,omitempty"`
	State         JobState              `json:"state,omitempty"`
	FailureReason spec.JobFailureReason `json:"failure_reason,omitempty"`
	Checksum      string                `json:"checksum,omitempty"` // deprecated
	Output        JobTraceOutput        `json:"output,omitempty"`
	ExitCode      int                   `json:"exit_code,omitempty"`
}

type JobTraceOutput struct {
	Checksum string `json:"checksum,omitempty"`
	Bytesize int    `json:"bytesize,omitempty"`
}

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
	FailureReason spec.JobFailureReason
	Output        JobTraceOutput
	ExitCode      int
}

type RouterDiscovery struct {
	ServerURL string       `json:"server_url"`
	TLSData   spec.TLSData `json:"-"`
}

type FailuresCollector interface {
	RecordFailure(reason spec.JobFailureReason, runnerConfig RunnerConfig)
}

type SupportedFailureReasonMapper interface {
	Map(fr spec.JobFailureReason) spec.JobFailureReason
}

type JobTrace interface {
	io.Writer
	Success() error
	Fail(err error, failureData JobFailureData) error
	Finish()
	SetCancelFunc(cancelFunc context.CancelFunc)
	Cancel() bool
	SetAbortFunc(abortFunc context.CancelFunc)
	Abort() bool
	SetFailuresCollector(fc FailuresCollector)
	SetSupportedFailureReasonMapper(f SupportedFailureReasonMapper)
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

type ArtifactsOptions struct {
	BaseName           string
	ExpireIn           string
	Format             spec.ArtifactFormat
	Type               string
	LogResponseDetails bool
}

type Network interface {
	SetConnectionMaxAge(time.Duration)
	RegisterRunner(config RunnerConfig, parameters RegisterRunnerParameters) *RegisterRunnerResponse
	VerifyRunner(config RunnerConfig, systemID string) *VerifyRunnerResponse
	UnregisterRunner(config RunnerConfig) bool
	UnregisterRunnerManager(config RunnerConfig, systemID string) bool
	ResetToken(runner RunnerConfig, systemID string) *ResetTokenResponse
	ResetTokenWithPAT(runner RunnerConfig, systemID string, pat string) *ResetTokenResponse
	RequestJob(ctx context.Context, config RunnerConfig, sessionInfo *SessionInfo) (*spec.Job, bool)
	UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, content []byte,
		startOffset int, debugModeEnabled bool) PatchTraceResult
	DownloadArtifacts(config JobCredentials, artifactsFile io.WriteCloser, directDownload *bool) DownloadState
	UploadRawArtifacts(config JobCredentials, bodyProvider ContentProvider, options ArtifactsOptions) (UploadState, string)
	ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) (JobTrace, error)
}
