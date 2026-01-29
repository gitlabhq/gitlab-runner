package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/auth_methods"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

type JobFailureReason string

func (r JobFailureReason) String() string {
	return string(r)
}

type JobInfo struct {
	Name  string `json:"name"`
	Stage string `json:"stage"`

	ProjectID       int64  `json:"project_id"`
	ProjectName     string `json:"project_name"`
	ProjectFullPath string `json:"project_full_path"`

	NamespaceID     int64 `json:"namespace_id"`
	RootNamespaceID int64 `json:"root_namespace_id"`
	OrganizationID  int64 `json:"organization_id"`

	InstanceID   string `json:"instance_id"`
	InstanceUUID string `json:"instance_uuid"`

	UserID       int64  `json:"user_id"`
	ScopedUserID *int64 `json:"scoped_user_id,omitempty"`

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
	Protected        *bool          `json:"protected"`
}

type Variable struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Public   bool   `json:"public"`
	Internal bool   `json:"-"`
	File     bool   `json:"file"`
	Masked   bool   `json:"masked"`
	Raw      bool   `json:"raw"`
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
	Script       StepScript `json:"script" inputs:"expand"`
	Timeout      int        `json:"timeout"`
	When         StepWhen   `json:"when"`
	AllowFailure bool       `json:"allow_failure"`
}

func (s *Step) Expand(inputs *Inputs) error {
	switch s.Name {
	case StepNameScript:
	case StepNameAfterScript:
	default:
		// Step name not supported
		return nil
	}

	type alias Step
	return ExpandInputs(inputs, (*alias)(s))
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

var supportedExecutorOptions = map[string][]string{
	"docker":     {"platform", "user"},
	"kubernetes": {"user"},
}

type (
	ImageDockerOptions struct {
		executorOptions
		Platform string        `json:"platform" inputs:"expand"`
		User     StringOrInt64 `json:"user" inputs:"expand"`
	}

	StringOrInt64 string

	ImageKubernetesOptions struct {
		executorOptions
		User StringOrInt64 `json:"user" inputs:"expand"`
	}
	ImageExecutorOptions struct {
		executorOptions
		Docker     ImageDockerOptions     `json:"docker,omitempty" inputs:"expand"`
		Kubernetes ImageKubernetesOptions `json:"kubernetes,omitempty" inputs:"expand"`
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
	ido.unsupportedOptions = ido.validate(data, supportedExecutorOptions["docker"], "docker executor", "image")
	return nil
}

func (ido *ImageDockerOptions) Expand(vars Variables) ImageDockerOptions {
	return ImageDockerOptions{
		Platform: vars.ExpandValue(ido.Platform),
		User:     StringOrInt64(vars.ExpandValue(string(ido.User))),
	}
}

func (iko *ImageKubernetesOptions) UnmarshalJSON(data []byte) error {
	type imageKubernetesOptions ImageKubernetesOptions
	inner := imageKubernetesOptions{}
	if err := json.Unmarshal(data, &inner); err != nil {
		return err
	}
	*iko = ImageKubernetesOptions(inner)

	// call validate after json.Unmarshal so the former handles bad json.
	iko.unsupportedOptions = iko.validate(data, supportedExecutorOptions["kubernetes"], "kubernetes executor", "image")
	return nil
}

func (iko *ImageKubernetesOptions) Expand(vars Variables) ImageKubernetesOptions {
	return ImageKubernetesOptions{
		User: StringOrInt64(vars.ExpandValue(string(iko.User))),
	}
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
	ieo.unsupportedOptions = ieo.validate(data, mapKeys(supportedExecutorOptions), "executor_opts", "image")
	return nil
}

func (ieo *ImageExecutorOptions) UnsupportedOptions() error {
	return errors.Join(
		ieo.executorOptions.UnsupportedOptions(),
		ieo.Docker.UnsupportedOptions(),
		ieo.Kubernetes.UnsupportedOptions(),
	)
}

type PullPolicy string

type Image struct {
	Name            string               `json:"name" inputs:"expand"`
	Alias           string               `json:"alias,omitempty"`
	Command         []string             `json:"command,omitempty" inputs:"expand"`
	Entrypoint      []string             `json:"entrypoint,omitempty" inputs:"expand"`
	Ports           []Port               `json:"ports,omitempty"`
	Variables       Variables            `json:"variables,omitempty"`
	PullPolicies    []PullPolicy         `json:"pull_policy,omitempty" inputs:"expand"`
	ExecutorOptions ImageExecutorOptions `json:"executor_opts,omitempty" inputs:"expand"`
}

func (i *Image) Aliases() []string { return strings.Fields(strings.ReplaceAll(i.Alias, ",", " ")) }

func (i *Image) UnsupportedOptions() error {
	return i.ExecutorOptions.UnsupportedOptions()
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
	Name      string          `json:"name" inputs:"expand"`
	Untracked bool            `json:"untracked"`
	Paths     ArtifactPaths   `json:"paths" inputs:"expand"`
	Exclude   ArtifactExclude `json:"exclude" inputs:"expand"`
	When      ArtifactWhen    `json:"when" inputs:"expand"`
	Type      string          `json:"artifact_type"`
	Format    ArtifactFormat  `json:"artifact_format"`
	ExpireIn  string          `json:"expire_in" inputs:"expand"`
}

type Artifacts []Artifact

type PolicyOptions struct {
	PolicyJob                  bool     `json:"execution_policy_job"`
	Name                       string   `json:"policy_name"`
	VariableOverrideAllowed    *bool    `json:"policy_variables_override_allowed,omitempty"`
	VariableOverrideExceptions []string `json:"policy_variables_override_exceptions,omitempty"`
}

type Cache struct {
	Key          string            `json:"key" inputs:"expand"`
	Untracked    bool              `json:"untracked"`
	Policy       CachePolicy       `json:"policy" inputs:"expand"`
	Paths        ArtifactPaths     `json:"paths" inputs:"expand"`
	When         CacheWhen         `json:"when" inputs:"expand"`
	FallbackKeys CacheFallbackKeys `json:"fallback_keys" inputs:"expand"`
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

type TLSData struct {
	CAChain  string `json:"-"`
	AuthCert string `json:"-"`
	AuthKey  string `json:"-"`
}

type Job struct {
	ID            int64          `json:"id"`
	Token         string         `json:"token"`
	AllowGitFetch bool           `json:"allow_git_fetch"`
	JobInfo       JobInfo        `json:"job_info"`
	GitInfo       GitInfo        `json:"git_info"`
	RunnerInfo    RunnerInfo     `json:"runner_info"`
	Inputs        Inputs         `json:"inputs"`
	Variables     Variables      `json:"variables"`
	Steps         Steps          `json:"steps" inputs:"expand"`
	Image         Image          `json:"image" inputs:"expand"`
	Services      Services       `json:"services" inputs:"expand"`
	Artifacts     Artifacts      `json:"artifacts" inputs:"expand"`
	Cache         Caches         `json:"cache" inputs:"expand"`
	Credentials   []Credentials  `json:"credentials"`
	Dependencies  Dependencies   `json:"dependencies"`
	Features      GitlabFeatures `json:"features"`
	Secrets       Secrets        `json:"secrets,omitempty"`
	Hooks         Hooks          `json:"hooks,omitempty"`
	Run           Run            `json:"run,omitempty"`
	PolicyOptions PolicyOptions  `json:"policy_options,omitempty"`

	TLSData TLSData `json:"-"`

	JobRequestCorrelationID string `json:"-"`
}

type Run []schema.Step

func (r *Run) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var run []schema.Step
	if err := json.Unmarshal([]byte(s), &run); err != nil {
		return err
	}

	*r = run
	return nil
}

// ValidateStepsJobRequest does the following:
// 1. It detects if the JobRequest is requesting execution of the job via Steps.
// 2. If yes, it ensures the request is a valid steps request, and
// 3. It sets a default build image.
// 4. It further determines if the request is a valid native steps execution request.
// 5. If it is, it sets a new, native-steps specific script step and returns.
// 6. If not, it configures the job to be run via the step shim approach.
func (j *Job) ValidateStepsJobRequest(executorSupportsNativeSteps bool) error {
	switch {
	case len(j.Run) == 0:
		return nil
	case slices.ContainsFunc(j.Steps, func(step Step) bool { return len(step.Script) > 0 }):
		return fmt.Errorf("the `run` and `script` keywords cannot be used together")
	case j.Variables.Get("STEPS") != "":
		return fmt.Errorf("the `run` keyword requires the exclusive use of the variable STEPS")
	}

	if executorSupportsNativeSteps {
		// If the executor supports native step execution and the job was specified as steps, execute the job via native
		// steps integration. In other words, disallow executing the job in shim mode if the executor supports native
		// steps.

		// If native steps is enabled, the script steps won't be executed anyway, but this change ensures the job log
		// trace is coherent since it will print: Executing "step_run" stage of the job script
		j.Steps = Steps{{Name: StepNameRun}}

		return nil
	}

	// re-encode the run steps to a string for shim-mode
	runStr, _ := json.Marshal(j.Run)

	// Use the shim approach to run steps jobs. This shims GitLab Steps from the `run` keyword into the step-runner
	// image. This is a temporary mechanism for executing steps which will be replaced by a gRPC connection to
	// step-runner in each executor.
	j.Variables = append(j.Variables, Variable{
		Key:   "STEPS",
		Value: string(runStr),
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

type Secrets map[string]Secret

type Secret struct {
	Vault                *VaultSecret                `json:"vault,omitempty"`
	GCPSecretManager     *GCPSecretManagerSecret     `json:"gcp_secret_manager,omitempty"`
	AzureKeyVault        *AzureKeyVaultSecret        `json:"azure_key_vault,omitempty"`
	AWSSecretsManager    *AWSSecret                  `json:"aws_secrets_manager,omitempty"`
	GitLabSecretsManager *GitLabSecretsManagerSecret `json:"gitlab_secrets_manager,omitempty"`
	File                 *bool                       `json:"file,omitempty"`
}

func (s Secrets) ExpandVariables(vars Variables) {
	for _, secret := range s {
		secret.ExpandVariables(vars)
	}
}

func (s Secret) ExpandVariables(vars Variables) {
	if s.Vault != nil {
		s.Vault.expandVariables(vars)
	}
	if s.GCPSecretManager != nil {
		s.GCPSecretManager.expandVariables(vars)
	}
	if s.AzureKeyVault != nil {
		s.AzureKeyVault.expandVariables(vars)
	}
	if s.AWSSecretsManager != nil {
		s.AWSSecretsManager.expandVariables(vars)
	}
	// NOTE: GitLab Secrets Manager doesn't support variable expansion
	// The only user input from the CI config is the secret name which Rails
	// transforms into the path. Everything else is generated internally by Rails.
}

// IsFile defines whether the variable should be of type FILE or no.
//
// The default behavior is to represent the variable as FILE type.
// If defined by the user - set to whatever was chosen.
func (s Secret) IsFile() bool {
	if s.File == nil {
		return true
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

func (s *AWSSecret) expandVariables(vars Variables) {
	s.SecretId = vars.ExpandValue(s.SecretId)
	s.VersionId = vars.ExpandValue(s.VersionId)
	s.VersionStage = vars.ExpandValue(s.VersionStage)
	s.Field = vars.ExpandValue(s.Field)
	s.Region = vars.ExpandValue(s.Region)
	s.RoleARN = vars.ExpandValue(s.RoleARN)
	s.RoleSessionName = vars.ExpandValue(s.RoleSessionName)
	s.Server.expandVariables(vars)
}

func (s *AWSServer) expandVariables(vars Variables) {
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

func (s *GCPSecretManagerSecret) expandVariables(vars Variables) {
	s.Name = vars.ExpandValue(s.Name)
	s.Version = vars.ExpandValue(s.Version)

	s.Server.expandVariables(vars)
}

func (s *GCPSecretManagerServer) expandVariables(vars Variables) {
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

func (s *AzureKeyVaultSecret) expandVariables(vars Variables) {
	s.Server.expandVariables(vars)

	s.Name = vars.ExpandValue(s.Name)
	s.Version = vars.ExpandValue(s.Version)
}

func (s *AzureKeyVaultServer) expandVariables(vars Variables) {
	s.JWT = vars.ExpandValue(s.JWT)
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

func (s *VaultSecret) expandVariables(vars Variables) {
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

func (s *VaultServer) expandVariables(vars Variables) {
	s.URL = vars.ExpandValue(s.URL)
	s.Namespace = vars.ExpandValue(s.Namespace)

	s.Auth.expandVariables(vars)
}

func (a *VaultAuth) expandVariables(vars Variables) {
	a.Name = vars.ExpandValue(a.Name)
	a.Path = vars.ExpandValue(a.Path)

	for field, value := range a.Data {
		a.Data[field] = vars.ExpandValue(fmt.Sprintf("%s", value))
	}
}

func (e *VaultEngine) expandVariables(vars Variables) {
	e.Name = vars.ExpandValue(e.Name)
	e.Path = vars.ExpandValue(e.Path)
}

// GitLabSecretsManagerSecret represents a secret configuration for GitLab's native
// secrets management system using OpenBao as the backend.
type GitLabSecretsManagerSecret struct {
	Server GitLabSecretsManagerServer `json:"server"`
	Engine GitLabSecretsManagerEngine `json:"engine"`
	Path   string                     `json:"path"`
	Field  string                     `json:"field"`
}

// GitLabSecretsManagerServer contains the configuration for connecting to the
// OpenBao server and authenticating via JWT.
type GitLabSecretsManagerServer struct {
	URL        string                               `json:"url"`
	InlineAuth GitLabSecretsManagerServerInlineAuth `json:"inline_auth"`
}

// GitLabSecretsManagerServerInlineAuth holds the inline authentication configuration
// for OpenBao JWT authentication. This allows the runner to authenticate on each
// request without storing tokens.
type GitLabSecretsManagerServerInlineAuth struct {
	// Path is the full path for this login request. This is assumed to be
	// against an OpenBao auth method that takes a role and jwt parameter;
	// or, roughly equivalent semantic as the JWT auth engine.
	Path string `json:"path"`

	// JWT is the JWT to use to authenticate against the OpenBao server.
	JWT string `json:"jwt"`

	// Role is the required login authentication role.
	Role string `json:"role"`

	// AuthMount is a legacy field sent on older GitLab versions and must be
	// templated to auth/<auth_mount>/login. Newer server versions send the
	// full request path to authenticate via.
	AuthMount string `json:"auth_mount"`
}

// GitLabSecretsManagerEngine specifies the secret engine configuration in OpenBao,
// including the engine type and mount path.
type GitLabSecretsManagerEngine struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (j *Job) RepoCleanURL() string {
	return url_helpers.CleanURL(j.GitInfo.RepoURL)
}

func (j *Job) JobURL() string {
	url := strings.TrimSuffix(j.RepoCleanURL(), ".git")

	return fmt.Sprintf("%s/-/jobs/%d", url, j.ID)
}

func (j *Job) UnsupportedOptions() error {
	return errors.Join(
		j.Image.UnsupportedOptions(),
		j.Services.UnsupportedOptions(),
	)
}
