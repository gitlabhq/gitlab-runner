package common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
)

type (
	UpdateState      int
	PatchState       int
	UploadState      int
	DownloadState    int
	JobState         string
	JobFailureReason string
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

func (r JobFailureReason) String() string {
	return string(r)
}

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
	ImagePullFailure    JobFailureReason = "image_pull_failure"
	UnknownFailure      JobFailureReason = "unknown_failure"

	// ConfigurationError indicates an error in the CI configuration that can only be determined by runner (and not by
	// Rails). The typical example incompatible pull policies. Since this failure reason does not exist in rails, we map
	// it to ScriptFailure below, which is more or less correct in that it's ultimately a user error.
	ConfigurationError JobFailureReason = "configuration_error"

	// When defining new job failure reasons, consider if its meaning is
	// extracted from the scope of already existing one. If yes - update
	// the failureReasonsCompatibilityMap variable below.

	// Always update the allFailureReasons list

	// JobCanceled is only internal to runner, and not used inside of rails.
	JobCanceled JobFailureReason = "job_canceled"
)

var (
	// allFailureReasons contains the list of all failure reasons known to runner.
	allFailureReasons = []JobFailureReason{
		ScriptFailure,
		RunnerSystemFailure,
		JobExecutionTimeout,
		ImagePullFailure,
		UnknownFailure,
		ConfigurationError,
	}

	// failureReasonsCompatibilityMap contains a mapping of new failure reasons
	// to old failure reasons.
	//
	// Some new failure reasons may be extracted from old failure reasons
	// If they are not yet recognized by GitLab, we may try to use the older, wider
	// category for them (yet we still need to pass the that value through
	// supported list check).
	failureReasonsCompatibilityMap = map[JobFailureReason]JobFailureReason{
		ImagePullFailure:   RunnerSystemFailure,
		ConfigurationError: ScriptFailure,
	}

	// A small list of failure reasons that are supported by all
	// GitLab instances.
	alwaysSupportedFailureReasons = []JobFailureReason{
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

	TimeInQueueSeconds                       float64 `json:"time_in_queue_seconds"`
	ProjectJobsRunningOnInstanceRunnersCount string  `json:"project_jobs_running_on_instance_runners_count"`
	QueueSize                                int64   `json:"queue_size"`
	QueueDepth                               int64   `json:"queue_depth"`
}

type GitInfoRefType string

const (
	RefTypeBranch GitInfoRefType = "branch"
	RefTypeTag    GitInfoRefType = "tag"
)

type GitInfo struct {
	RepoURL          string         `json:"repo_url"`
	RepoObjectFormat string         `json:"repo_object_format"`
	Ref              string         `json:"ref"`
	Sha              string         `json:"sha"`
	BeforeSha        string         `json:"before_sha"`
	RefType          GitInfoRefType `json:"ref_type"`
	Refspecs         []string       `json:"refspecs"`
	Depth            int            `json:"depth"`
}

type RunnerInfo struct {
	Timeout int `json:"timeout"`
}

type StepScript []string

type StepName string

const (
	StepNameRun         StepName = "run"
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

type (
	UnsuportedExecutorOptionsError struct {
		executor, section                    string
		unsupportedOptions, supportedOptions []string
	}
	executorOptions struct {
		unsupportedOptions error
	}
)

func (ueoe *UnsuportedExecutorOptionsError) Error() string {
	return fmt.Sprintf("Unsupported %q options %v for %q; supported options are %v",
		ueoe.section,
		ueoe.unsupportedOptions,
		ueoe.executor,
		ueoe.supportedOptions)
}

func (eo *executorOptions) validate(data []byte, supportedOptions []string, executor, section string) error {
	options := map[string]any{}
	if err := json.Unmarshal(data, &options); err != nil {
		// this can't happen
		return nil
	}

	notSupported := []string{}
	for opt := range options {
		if !slices.Contains(supportedOptions, opt) {
			notSupported = append(notSupported, opt)
		}
	}
	if len(notSupported) != 0 {
		sort.Strings(supportedOptions)

		return &UnsuportedExecutorOptionsError{
			executor:           executor,
			section:            section,
			unsupportedOptions: notSupported,
			supportedOptions:   supportedOptions,
		}
	}

	return nil
}

func (eo *executorOptions) UnsupportedOptions() error {
	return eo.unsupportedOptions
}

var SupportedExecutorOptions = map[string][]string{
	"docker":     {"platform", "user"},
	"kubernetes": {"user"},
}

type (
	ImageDockerOptions struct {
		executorOptions
		Platform string        `json:"platform"`
		User     StringOrInt64 `json:"user"`
	}

	StringOrInt64 string

	ImageKubernetesOptions struct {
		executorOptions
		User StringOrInt64 `json:"user"`
	}
	ImageExecutorOptions struct {
		executorOptions
		Docker     ImageDockerOptions     `json:"docker,omitempty"`
		Kubernetes ImageKubernetesOptions `json:"kubernetes,omitempty"`
	}
)

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (ido *ImageDockerOptions) UnmarshalJSON(data []byte) error {
	type imageDockerOptions ImageDockerOptions
	inner := imageDockerOptions{}
	if err := json.Unmarshal(data, &inner); err != nil {
		return err
	}
	*ido = ImageDockerOptions(inner)

	// call validate after json.Unmarshal so the former handles bad json.
	ido.unsupportedOptions = ido.validate(data, SupportedExecutorOptions["docker"], "docker executor", "image")
	return nil
}

func (iko *ImageKubernetesOptions) UnmarshalJSON(data []byte) error {
	type imageKubernetesOptions ImageKubernetesOptions
	inner := imageKubernetesOptions{}
	if err := json.Unmarshal(data, &inner); err != nil {
		return err
	}
	*iko = ImageKubernetesOptions(inner)

	// call validate after json.Unmarshal so the former handles bad json.
	iko.unsupportedOptions = iko.validate(data, SupportedExecutorOptions["kubernetes"], "kubernetes executor", "image")
	return nil
}

func (iko *ImageKubernetesOptions) GetUIDGID() (int64, int64, error) {
	if iko.User == "" {
		return 0, 0, nil
	}

	user, group, ok := strings.Cut(string(iko.User), ":")

	uid, err := strconv.ParseInt(user, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse UID %w", err)
	}

	var gid int64
	if ok {
		gid, err = strconv.ParseInt(group, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse GID %w", err)
		}
	}

	return uid, gid, err
}

func (si *StringOrInt64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*si = StringOrInt64(s)
		return nil
	}

	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*si = StringOrInt64(strconv.FormatInt(i, 10))
		return nil
	}

	return fmt.Errorf("StringOrInt: input not string or integer")
}

func (ieo *ImageExecutorOptions) UnmarshalJSON(data []byte) error {
	type imageExecutorOptions ImageExecutorOptions
	inner := imageExecutorOptions{}
	if err := json.Unmarshal(data, &inner); err != nil {
		return err
	}
	*ieo = ImageExecutorOptions(inner)

	// call validate after json.Unmarshal so the former handles bad json.
	ieo.unsupportedOptions = ieo.validate(data, mapKeys(SupportedExecutorOptions), "executor_opts", "image")
	return nil
}

func (ieo *ImageExecutorOptions) UnsupportedOptions() error {
	return errors.Join(
		ieo.executorOptions.UnsupportedOptions(),
		ieo.Docker.UnsupportedOptions(),
		ieo.Kubernetes.UnsupportedOptions(),
	)
}

type Image struct {
	Name            string               `json:"name"`
	Alias           string               `json:"alias,omitempty"`
	Command         []string             `json:"command,omitempty"`
	Entrypoint      []string             `json:"entrypoint,omitempty"`
	Ports           []Port               `json:"ports,omitempty"`
	Variables       JobVariables         `json:"variables,omitempty"`
	PullPolicies    []DockerPullPolicy   `json:"pull_policy,omitempty"`
	ExecutorOptions ImageExecutorOptions `json:"executor_opts,omitempty"`
}

func (i *Image) Aliases() []string { return strings.Fields(strings.ReplaceAll(i.Alias, ",", " ")) }

func (i *Image) UnsupportedOptions() error {
	return i.ExecutorOptions.UnsupportedOptions()
}

func (i *Image) LogFields() logrus.Fields {
	// Empty Name means the field is in fact not used. So whatever wants to
	// use this method to prepare information to logging, nil response means
	// there is no need to log at all.
	if i.Name == "" {
		return nil
	}

	fields := logrus.Fields{
		"image_name": i.Name,
	}

	if i.ExecutorOptions.Docker.Platform != "" {
		fields["image_platform"] = i.ExecutorOptions.Docker.Platform
	}

	return fields
}

type Port struct {
	Number   int    `json:"number,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
}

type Services []Image

func (s Services) UnsupportedOptions() error {
	errs := make([]error, 0, len(s))
	for _, i := range s {
		errs = append(errs, i.UnsupportedOptions())
	}
	return errors.Join(errs...)
}

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
	ArtifactFormatZipZstd ArtifactFormat = "zipzstd"
	ArtifactFormatTarZstd ArtifactFormat = "tarzstd"
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

type PolicyOptions struct {
	PolicyJob                  bool     `json:"execution_policy_job"`
	Name                       string   `json:"policy_name"`
	VariableOverrideAllowed    *bool    `json:"policy_variables_override_allowed,omitempty"`
	VariableOverrideExceptions []string `json:"policy_variables_override_exceptions,omitempty"`
}

type Cache struct {
	Key          string            `json:"key"`
	Untracked    bool              `json:"untracked"`
	Policy       CachePolicy       `json:"policy"`
	Paths        ArtifactPaths     `json:"paths"`
	When         CacheWhen         `json:"when"`
	FallbackKeys CacheFallbackKeys `json:"fallback_keys"`
}

type (
	CacheWhen         string
	CacheFallbackKeys []string
)

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
	Run           string         `json:"run"`
	PolicyOptions PolicyOptions  `json:"policy_options,omitempty"`

	TLSCAChain  string `json:"-"`
	TLSAuthCert string `json:"-"`
	TLSAuthKey  string `json:"-"`

	JobRequestCorrelationID string `json:"-"`
}

// ValidateStepsJobRequest does the following:
// 1. It detects if the JobRequest is requesting execution of the job via Steps.
// 2. If yes, it ensures the request is a valid steps request, and
// 3. It sets a default build image.
// 4. It further determines if the request is a valid native steps execution request.
// 5. If it is, it sets a new, native-steps specific script step and returns.
// 6. If not, it configures the job to be run via the step shim approach.
func (j *JobResponse) ValidateStepsJobRequest(executorSupportsNativeSteps bool) error {
	switch {
	case j.Run == "":
		return nil
	case slices.ContainsFunc(j.Steps, func(step Step) bool { return len(step.Script) > 0 }):
		return fmt.Errorf("the `run` and `script` keywords cannot be used together")
	case j.Variables.Get("STEPS") != "":
		return fmt.Errorf("the `run` keyword requires the exclusive use of the variable STEPS")
	}

	if j.Image.Name == "" {
		// Experiment requires step-runner to be present in
		// the container image. If no image is provided then
		// we use the step-runner v0 image.
		j.Image.Name = "registry.gitlab.com/gitlab-org/step-runner:v0"
	}

	if executorSupportsNativeSteps && j.NativeStepsRequested() {
		// If native steps is enabled, the script steps won't be executed anyway, but this change ensures the job log
		// trace is coherent since it will print: Executing "step_run" stage of the job script
		j.Steps = Steps{{Name: StepNameRun}}

		return nil
	}

	// Use the shim approach to run steps jobs. This shims GitLab Steps from the `run` keyword into the step-runner
	// image. This is a temporary mechanism for executing steps which will be replaced by a gRPC connection to
	// step-runner in each executor.
	j.Variables = append(j.Variables, JobVariable{
		Key:   "STEPS",
		Value: j.Run,
		Raw:   true,
	})

	j.Steps = Steps{{
		Name:         StepNameScript,
		Script:       StepScript{"step-runner ci"},
		Timeout:      3600,
		When:         "on_success",
		AllowFailure: false,
	}}

	return nil
}

func (j *JobResponse) NativeStepsRequested() bool {
	if j.Run == "" {
		return false
	}
	for _, v := range j.Variables {
		if v.Key == featureflags.UseNativeSteps {
			return true
		}
	}
	return false
}

type Secrets map[string]Secret

type Secret struct {
	Vault             *VaultSecret            `json:"vault,omitempty"`
	GCPSecretManager  *GCPSecretManagerSecret `json:"gcp_secret_manager,omitempty"`
	AzureKeyVault     *AzureKeyVaultSecret    `json:"azure_key_vault,omitempty"`
	Akeyless          *AkeylessSecret         `json:"akeyless,omitempty"`
	AWSSecretsManager *AWSSecret              `json:"aws_secrets_manager,omitempty"`
	File              *bool                   `json:"file,omitempty"`
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
	if s.GCPSecretManager != nil {
		s.GCPSecretManager.expandVariables(vars)
	}
	if s.AzureKeyVault != nil {
		s.AzureKeyVault.expandVariables(vars)
	}
	if s.Akeyless != nil {
		s.Akeyless.expandVariables(vars)
	}
	if s.AWSSecretsManager != nil {
		s.AWSSecretsManager.expandVariables(vars)
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

type GCPSecretManagerSecret struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Server  GCPSecretManagerServer `json:"server"`
}

type GCPSecretManagerServer struct {
	ProjectNumber                        string `json:"project_number"`
	WorkloadIdentityFederationPoolId     string `json:"workload_identity_federation_pool_id"`
	WorkloadIdentityFederationProviderID string `json:"workload_identity_federation_provider_id"`
	JWT                                  string `json:"jwt"`
}

type AWSSecret struct {
	SecretId        string    `json:"secret_id"`
	VersionId       string    `json:"version_id,omitempty"`
	VersionStage    string    `json:"version_stage,omitempty"`
	Field           string    `json:"field,omitempty"`
	Region          string    `json:"region,omitempty"`
	RoleARN         string    `json:"role_arn,omitempty"`
	RoleSessionName string    `json:"role_session_name,omitempty"`
	Server          AWSServer `json:"server,omitempty"`
}

type AWSServer struct {
	Region          string `json:"region"`
	JWT             string `json:"jwt,omitempty"`
	RoleArn         string `json:"role_arn,omitempty"`
	RoleSessionName string `json:"role_session_name,omitempty"`
}

func (s *AWSSecret) expandVariables(vars JobVariables) {
	s.SecretId = vars.ExpandValue(s.SecretId)
	s.VersionId = vars.ExpandValue(s.VersionId)
	s.VersionStage = vars.ExpandValue(s.VersionStage)
	s.Field = vars.ExpandValue(s.Field)
	s.Region = vars.ExpandValue(s.Region)
	s.RoleARN = vars.ExpandValue(s.RoleARN)
	s.RoleSessionName = vars.ExpandValue(s.RoleSessionName)
	s.Server.expandVariables(vars)
}

func (s *AWSServer) expandVariables(vars JobVariables) {
	s.JWT = vars.ExpandValue(s.JWT)
	s.Region = vars.ExpandValue(s.Region)
	s.RoleArn = vars.ExpandValue(s.RoleArn)
	if s.RoleSessionName == "" {
		s.RoleSessionName = "${CI_JOB_ID}-${CI_PROJECT_ID}-${CI_SERVER_HOST}"
	}
	s.RoleSessionName = vars.ExpandValue(s.RoleSessionName)
	if len(s.RoleSessionName) > 64 {
		s.RoleSessionName = s.RoleSessionName[:64]
	}
}

func (s *GCPSecretManagerSecret) expandVariables(vars JobVariables) {
	s.Name = vars.ExpandValue(s.Name)
	s.Version = vars.ExpandValue(s.Version)

	s.Server.expandVariables(vars)
}

func (s *GCPSecretManagerServer) expandVariables(vars JobVariables) {
	s.ProjectNumber = vars.ExpandValue(s.ProjectNumber)
	s.WorkloadIdentityFederationPoolId = vars.ExpandValue(s.WorkloadIdentityFederationPoolId)
	s.WorkloadIdentityFederationProviderID = vars.ExpandValue(s.WorkloadIdentityFederationProviderID)
	s.JWT = vars.ExpandValue(s.JWT)
}

type AzureKeyVaultSecret struct {
	Name    string              `json:"name"`
	Version string              `json:"version,omitempty"`
	Server  AzureKeyVaultServer `json:"server"`
}

type AzureKeyVaultServer struct {
	ClientID string `json:"client_id"`
	TenantID string `json:"tenant_id"`
	JWT      string `json:"jwt"`
	URL      string `json:"url"`
}

func (s *AzureKeyVaultSecret) expandVariables(vars JobVariables) {
	s.Server.expandVariables(vars)

	s.Name = vars.ExpandValue(s.Name)
	s.Version = vars.ExpandValue(s.Version)
}

func (s *AzureKeyVaultServer) expandVariables(vars JobVariables) {
	s.JWT = vars.ExpandValue(s.JWT)
}

type AkeylessSecret struct {
	Name          string         `json:"name"`
	DataKey       string         `json:"data_key"`
	CertUserName  string         `json:"cert_user_name"`
	PublicKeyData string         `json:"public_key_data"`
	CsrData       string         `json:"csr_data"`
	Server        AkeylessServer `json:"server"`
}

type AkeylessServer struct {
	AccessId               string `json:"access_id"`
	AccessKey              string `json:"access_key"`
	AkeylessApiUrl         string `json:"akeyless_api_url"`
	AkeylessAccessType     string `json:"akeyless_access_type"`
	AkeylessToken          string `json:"akeyless_token"`
	JWT                    string `json:"jwt"`
	UidToken               string `json:"uid_token"`
	GcpAudience            string `json:"gcp_audience"`
	AzureObjectId          string `json:"azure_object_id"`
	K8SServiceAccountToken string `json:"k8s_service_account_token"`
	K8SAuthConfigName      string `json:"k8s_auth_config_name"`
	GatewayCaCert          string `json:"gateway_ca_certificate"`
}

func (s *AkeylessSecret) expandVariables(vars JobVariables) {
	s.Name = vars.ExpandValue(s.Name)
	s.DataKey = vars.ExpandValue(s.DataKey)
	s.CertUserName = vars.ExpandValue(s.CertUserName)
	s.PublicKeyData = vars.ExpandValue(s.PublicKeyData)
	s.CsrData = vars.ExpandValue(s.CsrData)
	s.Server.expandVariables(vars)
}

func (s *AkeylessServer) expandVariables(vars JobVariables) {
	s.JWT = vars.ExpandValue(s.JWT)
	s.AkeylessApiUrl = vars.ExpandValue(s.AkeylessApiUrl)
	s.AkeylessAccessType = vars.ExpandValue(s.AkeylessAccessType)
	s.AkeylessToken = vars.ExpandValue(s.AkeylessToken)
	s.UidToken = vars.ExpandValue(s.UidToken)
	s.GcpAudience = vars.ExpandValue(s.GcpAudience)
	s.AzureObjectId = vars.ExpandValue(s.AzureObjectId)
	s.K8SServiceAccountToken = vars.ExpandValue(s.K8SServiceAccountToken)
	s.K8SAuthConfigName = vars.ExpandValue(s.K8SAuthConfigName)
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

func (j *JobResponse) UnsupportedOptions() error {
	return errors.Join(
		j.Image.UnsupportedOptions(),
		j.Services.UnsupportedOptions(),
	)
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
	BaseName           string
	ExpireIn           string
	Format             ArtifactFormat
	Type               string
	LogResponseDetails bool
}

type FailuresCollector interface {
	RecordFailure(reason JobFailureReason, runnerConfig RunnerConfig)
}

type SupportedFailureReasonMapper interface {
	Map(fr JobFailureReason) JobFailureReason
}

type JobTrace interface {
	io.Writer
	Success() error
	Fail(err error, failureData JobFailureData) error
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

type Network interface {
	SetConnectionMaxAge(time.Duration)
	RegisterRunner(config RunnerCredentials, parameters RegisterRunnerParameters) *RegisterRunnerResponse
	VerifyRunner(config RunnerCredentials, systemID string) *VerifyRunnerResponse
	UnregisterRunner(config RunnerCredentials) bool
	UnregisterRunnerManager(config RunnerCredentials, systemID string) bool
	ResetToken(runner RunnerCredentials, systemID string) *ResetTokenResponse
	ResetTokenWithPAT(runner RunnerCredentials, systemID string, pat string) *ResetTokenResponse
	RequestJob(ctx context.Context, config RunnerConfig, sessionInfo *SessionInfo) (*JobResponse, bool)
	UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult
	PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, content []byte,
		startOffset int, debugModeEnabled bool) PatchTraceResult
	DownloadArtifacts(config JobCredentials, artifactsFile io.WriteCloser, directDownload *bool) DownloadState
	UploadRawArtifacts(config JobCredentials, bodyProvider ContentProvider, options ArtifactsOptions) (UploadState, string)
	ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) (JobTrace, error)
}
