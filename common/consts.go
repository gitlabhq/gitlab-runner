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
const AfterScriptTimeout = 5 * time.Minute
const DefaultMetricsServerPort = 9252
const DefaultCacheRequestTimeout = 10
const DefaultNetworkClientTimeout = 60 * time.Minute
const DefaultSessionTimeout = 30 * time.Minute
const WaitForBuildFinishTimeout = 5 * time.Minute

const (
	DefaultTraceOutputLimit    = 4 * 1024 * 1024 // in bytes
	DefaultTracePatchLimit     = 1024 * 1024     // in bytes
	DefaultTraceUpdateInterval = 3 * time.Second
	TraceFinishRetryInterval   = 3 * time.Second
	TraceForceSendInterval     = 30 * time.Second
)

var PreparationRetryInterval = 3 * time.Second

const (
	TestAlpineImage       = "alpine:3.7"
	TestWindowsImage      = "mcr.microsoft.com/windows/servercore:%s"
	TestAlpineNoRootImage = "registry.gitlab.com/gitlab-org/gitlab-runner/alpine-no-root:latest"
	TestDockerDindImage   = "docker:18-dind"
	TestDockerGitImage    = "docker:18-git"
)
