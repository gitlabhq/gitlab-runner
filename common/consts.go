package common

import "time"

const DefaultTimeout = 7200
const DefaultExecTimeout = 1800
const CheckInterval = 3 * time.Second
const NotHealthyCheckInterval = 300
const ReloadConfigInterval = 3
const HealthyChecks = 3
const HealthCheckInterval = 3600
const DefaultWaitForServicesTimeout = 30
const ShutdownTimeout = 30
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

const (
	DefaultTraceOutputLimit = 4 * 1024 * 1024 // in bytes
	DefaultTracePatchLimit  = 1024 * 1024     // in bytes

	DefaultUpdateInterval = 3 * time.Second
	MaxUpdateInterval     = 15 * time.Minute

	MinTraceForceSendInterval              = 30 * time.Second
	MaxTraceForceSendInterval              = 30 * time.Minute
	TraceForceSendUpdateIntervalMultiplier = 4
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
)
