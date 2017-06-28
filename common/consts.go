package common

import "time"

const DefaultTimeout = 7200
const DefaultExecTimeout = 1800
const CheckInterval = 3 * time.Second
const NotHealthyCheckInterval = 300
const UpdateInterval = 3 * time.Second
const UpdateRetryInterval = 3 * time.Second
const ReloadConfigInterval = 3
const HealthyChecks = 3
const HealthCheckInterval = 3600
const DefaultWaitForServicesTimeout = 30
const ShutdownTimeout = 30
const DefaultOutputLimit = 4096 // 4MB in kilobytes
const ForceTraceSentInterval = 30 * time.Second
const PreparationRetries = 3
const DefaultGetSourcesAttempts = 1
const DefaultArtifactDownloadAttempts = 1
const DefaultRestoreCacheAttempts = 1
const KubernetesPollInterval = 3
const KubernetesPollTimeout = 180
const AfterScriptTimeout = 5 * time.Minute
const DefaultMetricsServerPort = 9252
const DefaultCacheRequestTimeout = 10 * time.Minute

var PreparationRetryInterval = 3 * time.Second
