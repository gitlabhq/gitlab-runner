package common

import (
	"fmt"
	"time"
)

const DefaultTimeout = 7200
const DefaultExecTimeout = 1800
const DefaultCICDConfigFile = ".gitlab-ci.yml"
const CheckInterval = 3 * time.Second
const NotHealthyCheckInterval = 300
const ReloadConfigInterval = 3 * time.Second
const DefaultUnhealthyRequestsLimit = 3
const DefaultUnhealthyInterval = 60 * time.Minute
const DefaultWaitForServicesTimeout = 30
const DefaultShutdownTimeout = 30 * time.Second
const PreparationRetries = 3
const DefaultGetSourcesAttempts = 1
const DefaultArtifactDownloadAttempts = 1
const DefaultRestoreCacheAttempts = 1
const DefaultExecutorStageAttempts = 1
const KubernetesPollInterval = 3
const KubernetesPollTimeout = 180
const KubernetesResourceAvailabilityCheckMaxAttempts = 5
const AfterScriptTimeout = 5 * time.Minute
const DefaultMetricsServerPort = 9252
const DefaultCacheRequestTimeout = 10
const DefaultNetworkClientTimeout = 60 * time.Minute
const DefaultSessionTimeout = 30 * time.Minute
const WaitForBuildFinishTimeout = 5 * time.Minute
const SecretVariableDefaultsToFile = true
const TokenResetIntervalFactor = 0.75

const (
	DefaultTraceOutputLimit = 4 * 1024 * 1024 // in bytes
	DefaultTracePatchLimit  = 1024 * 1024     // in bytes

	DefaultUpdateInterval = 3 * time.Second
	MaxUpdateInterval     = 15 * time.Minute

	MinTraceForceSendInterval              = 30 * time.Second
	MaxTraceForceSendInterval              = 30 * time.Minute
	TraceForceSendUpdateIntervalMultiplier = 4

	// DefaultReaderBufferSize is the size of the line buffer.
	// Docker/Kubernetes use the same size to split lines
	DefaultReaderBufferSize = 16 * 1024
)

const (
	ExecutorKubernetes = "kubernetes"
)

var PreparationRetryInterval = 3 * time.Second

const (
	TestAlpineImage                 = "alpine:3.14.2"
	TestWindowsImage                = "mcr.microsoft.com/windows/servercore:%s"
	TestPwshImage                   = "mcr.microsoft.com/powershell:7.1.1-alpine-3.12-20210125"
	TestAlpineNoRootImage           = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-no-root:latest"
	TestAlpineEntrypointImage       = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-entrypoint:latest"
	TestAlpineEntrypointStderrImage = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-entrypoint-stderr:latest"
	TestHelperEntrypointImage       = "registry.gitlab.com/gitlab-org/gitlab-runner/helper-entrypoint:latest"
	TestAlpineIDOverflowImage       = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-id-overflow:latest"
	TestDockerDindImage             = "docker:18-dind"
	TestDockerGitImage              = "docker:18-git"
	TestLivenessImage               = "registry.gitlab.com/gitlab-org/ci-cd/tests/liveness:0.1.0"
	IncompatiblePullPolicy          = "pull_policy (%v) defined in %s is not one of the allowed_pull_policies (%v)"
)

type PullPolicySource string

const (
	PullPolicySourceGitLabCI = "GitLab pipeline config"
	PullPolicySourceRunner   = "Runner config"
	PullPolicySourceDefault  = "Runner config (default)"
)

// TODO: make pullPolicies and allowedPullPolicies generic on
// [T []DockerPullPolicy | []api.PullPolicy]
type incompatiblePullPolicyError struct {
	pullPolicySource    PullPolicySource
	pullPolicies        interface{}
	allowedPullPolicies interface{}
}

func (e *incompatiblePullPolicyError) Error() string {
	return fmt.Sprintf(IncompatiblePullPolicy, e.pullPolicies, string(e.pullPolicySource), e.allowedPullPolicies)
}

// TODO: make pullPolicies and allowedPullPolicies generic on
// [T []DockerPullPolicy | []api.PullPolicy]
func IncompatiblePullPolicyError(pullPolicy, allowedPullPolicies interface{}, pullPolicySource PullPolicySource) error {
	return &incompatiblePullPolicyError{
		pullPolicies:        pullPolicy,
		allowedPullPolicies: allowedPullPolicies,
		pullPolicySource:    pullPolicySource,
	}
}

type MaskOptions struct {
	Phrases       []string
	TokenPrefixes []string
}
