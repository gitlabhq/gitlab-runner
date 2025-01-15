package common

import (
	"fmt"
	"time"

	"github.com/go-http-utils/headers"
)

const DefaultTimeout = 7200
const DefaultExecTimeout = 1800
const DefaultCICDConfigFile = ".gitlab-ci.yml"
const CheckInterval = 3 * time.Second
const NotHealthyCheckInterval = 300
const ReloadConfigInterval = 3 * time.Second
const DefaultUnhealthyRequestsLimit = 3
const DefaultUnhealthyInterval = 60 * time.Minute
const DefaultfinalUpdateBackoffMax = 60 * time.Minute
const DefaultFinalUpdateRetryLimit = 10
const DefaultWaitForServicesTimeout = 30
const DefaultShutdownTimeout = 30 * time.Second
const PreparationRetries = 3
const DefaultGetSourcesAttempts = 1
const DefaultArtifactDownloadAttempts = 1
const DefaultRestoreCacheAttempts = 1
const DefaultExecutorStageAttempts = 1
const DefaultAfterScriptIgnoreErrors = true
const KubernetesPollInterval = 3
const KubernetesPollTimeout = 180
const KubernetesCleanupResourcesTimeout = 5 * time.Minute
const KubernetesResourceAvailabilityCheckMaxAttempts = 5
const AfterScriptTimeout = 5 * time.Minute
const DefaultMetricsServerPort = 9252
const DefaultCacheRequestTimeout = 10
const DefaultNetworkClientTimeout = 60 * time.Minute
const DefaultSessionTimeout = 30 * time.Minute
const WaitForBuildFinishTimeout = 5 * time.Minute
const SecretVariableDefaultsToFile = true
const TokenResetIntervalFactor = 0.75
const DefaultRequestRetryLimit = 5
const RequestRetryBackoffMin = 500 * time.Millisecond
const DefaultRequestRetryBackoffMax = 2000 * time.Millisecond

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
	TestDockerDindImage             = "docker:23-dind"
	TestDockerGitImage              = "docker:23-git"
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

// HTTP related constants
const (
	Accept                        = headers.Accept
	AcceptCharset                 = headers.AcceptCharset
	AcceptEncoding                = headers.AcceptEncoding
	AcceptLanguage                = headers.AcceptLanguage
	Authorization                 = headers.Authorization
	CacheControl                  = headers.CacheControl
	ContentLength                 = headers.ContentLength
	ContentMD5                    = headers.ContentMD5
	ContentType                   = headers.ContentType
	DoNotTrack                    = headers.DoNotTrack
	IfMatch                       = headers.IfMatch
	IfModifiedSince               = headers.IfModifiedSince
	IfNoneMatch                   = headers.IfNoneMatch
	IfRange                       = headers.IfRange
	IfUnmodifiedSince             = headers.IfUnmodifiedSince
	MaxForwards                   = headers.MaxForwards
	ProxyAuthorization            = headers.ProxyAuthorization
	Pragma                        = headers.Pragma
	Range                         = headers.Range
	Referer                       = headers.Referer
	UserAgent                     = headers.UserAgent
	TE                            = headers.TE
	Via                           = headers.Via
	Warning                       = headers.Warning
	Cookie                        = headers.Cookie
	Origin                        = headers.Origin
	AcceptDatetime                = headers.AcceptDatetime
	XRequestedWith                = headers.XRequestedWith
	AccessControlAllowOrigin      = headers.AccessControlAllowOrigin
	AccessControlAllowMethods     = headers.AccessControlAllowMethods
	AccessControlAllowHeaders     = headers.AccessControlAllowHeaders
	AccessControlAllowCredentials = headers.AccessControlAllowCredentials
	AccessControlExposeHeaders    = headers.AccessControlExposeHeaders
	AccessControlMaxAge           = headers.AccessControlMaxAge
	AccessControlRequestMethod    = headers.AccessControlRequestMethod
	AccessControlRequestHeaders   = headers.AccessControlRequestHeaders
	AcceptPatch                   = headers.AcceptPatch
	AcceptRanges                  = headers.AcceptRanges
	Allow                         = headers.Allow
	ContentEncoding               = headers.ContentEncoding
	ContentLanguage               = headers.ContentLanguage
	ContentLocation               = headers.ContentLocation
	ContentDisposition            = headers.ContentDisposition
	ContentRange                  = headers.ContentRange
	ETag                          = headers.ETag
	Expires                       = headers.Expires
	LastModified                  = headers.LastModified
	Link                          = headers.Link
	Location                      = headers.Location
	P3P                           = headers.P3P
	ProxyAuthenticate             = headers.ProxyAuthenticate
	Refresh                       = headers.Refresh
	RetryAfter                    = headers.RetryAfter
	Server                        = headers.Server
	SetCookie                     = headers.SetCookie
	StrictTransportSecurity       = headers.StrictTransportSecurity
	TransferEncoding              = headers.TransferEncoding
	Upgrade                       = headers.Upgrade
	Vary                          = headers.Vary
	WWWAuthenticate               = headers.WWWAuthenticate

	// Non-Standard
	XFrameOptions          = headers.XFrameOptions
	XXSSProtection         = headers.XXSSProtection
	ContentSecurityPolicy  = headers.ContentSecurityPolicy
	XContentSecurityPolicy = headers.XContentSecurityPolicy
	XWebKitCSP             = headers.XWebKitCSP
	XContentTypeOptions    = headers.XContentTypeOptions
	XPoweredBy             = headers.XPoweredBy
	XUACompatible          = headers.XUACompatible
	XForwardedProto        = headers.XForwardedProto
	XHTTPMethodOverride    = headers.XHTTPMethodOverride
	XForwardedFor          = headers.XForwardedFor
	XRealIP                = headers.XRealIP
	XCSRFToken             = headers.XCSRFToken
	XRatelimitLimit        = headers.XRatelimitLimit
	XRatelimitRemaining    = headers.XRatelimitRemaining
	XRatelimitReset        = headers.XRatelimitReset

	PrivateToken = "PRIVATE-TOKEN"
)
