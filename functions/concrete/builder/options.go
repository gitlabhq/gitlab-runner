package builder

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
)

type options struct {
	debug                         bool
	cacheDir                      string
	archiverStagingDir            string
	cloneURL                      string
	shell                         string
	loginShell                    bool
	artifactUploadTimeout         time.Duration
	artifactResponseHeaderTimeout time.Duration
	cacheArchiveFormat            string
	cacheMaxUploadArchiveSize     int64
	cacheUploadDescriptor         func(string) (cacheprovider.Descriptor, error)
	cacheDownloadDescriptor       func(string) (cacheprovider.Descriptor, error)
	preCloneScript                []string
	postCloneScript               []string
	preBuildScript                []string
	postBuildScript               []string
	safeDirectoryCheckout         bool
	isFeatureFlagOn               func(name string) bool
	gitCleanConfig                bool
	isSharedEnv                   bool
	userAgent                     string
	gitalyCorrelationID           string
	executorName                  string
	runnerName                    string
	startedAt                     time.Time
}

type Option func(*options) error

func newOptions(opts []Option) (options, error) {
	var options options

	options.cacheDir = "cache"
	options.shell = "sh"

	for _, o := range opts {
		err := o(&options)
		if err != nil {
			return options, err
		}
	}

	return options, nil
}

func WithIsSharedEnv(isShared bool) Option {
	return func(o *options) error {
		o.isSharedEnv = isShared
		return nil
	}
}

func WithCacheDir(dir string) Option {
	return func(o *options) error {
		o.cacheDir = dir
		return nil
	}
}

func WithArchiverStagingDir(dir string) Option {
	return func(o *options) error {
		o.archiverStagingDir = dir
		return nil
	}
}

func WithCloneURL(url string) Option {
	return func(o *options) error {
		o.cloneURL = url
		return nil
	}
}

func WithDebug(debug bool) Option {
	return func(o *options) error {
		o.debug = debug
		return nil
	}
}

func WithShell(shell string) Option {
	return func(o *options) error {
		o.shell = shell
		return nil
	}
}

func WithLoginShell(loginShell bool) Option {
	return func(o *options) error {
		o.loginShell = loginShell
		return nil
	}
}

func WithCacheMaxArchiveSize(size int64) Option {
	return func(o *options) error {
		o.cacheMaxUploadArchiveSize = size
		return nil
	}
}

func WithCacheUploadDescriptor(fn func(string) (cacheprovider.Descriptor, error)) Option {
	return func(o *options) error {
		o.cacheUploadDescriptor = fn
		return nil
	}
}

func WithCacheDownloadDescriptor(fn func(string) (cacheprovider.Descriptor, error)) Option {
	return func(o *options) error {
		o.cacheDownloadDescriptor = fn
		return nil
	}
}

func WithSafeDirectoryCheckout(safe bool) Option {
	return func(o *options) error {
		o.safeDirectoryCheckout = safe
		return nil
	}
}

func WithPreBuildScript(script []string) Option {
	return func(o *options) error {
		o.preBuildScript = script
		return nil
	}
}

func WithPostBuildScript(script []string) Option {
	return func(o *options) error {
		o.postBuildScript = script
		return nil
	}
}

func WithPreCloneScript(script []string) Option {
	return func(o *options) error {
		o.preCloneScript = script
		return nil
	}
}

func WithPostCloneScript(script []string) Option {
	return func(o *options) error {
		o.postCloneScript = script
		return nil
	}
}

func WithGitCleanConfig(enabled bool) Option {
	return func(o *options) error {
		o.gitCleanConfig = enabled
		return nil
	}
}

func WithFeatureFlagProvider(isFeatureFlagOn func(name string) bool) Option {
	return func(o *options) error {
		o.isFeatureFlagOn = isFeatureFlagOn
		return nil
	}
}

func WithUserAgent(userAgent string) Option {
	return func(o *options) error {
		o.userAgent = userAgent
		return nil
	}
}

func WithGitalyCorrelationID(correlationID string) Option {
	return func(o *options) error {
		o.gitalyCorrelationID = correlationID
		return nil
	}
}

func WithArtifactTimeouts(upload, responseHeader time.Duration) Option {
	return func(o *options) error {
		o.artifactUploadTimeout = upload
		o.artifactResponseHeaderTimeout = responseHeader
		return nil
	}
}

func WithExecutorName(name string) Option {
	return func(o *options) error {
		o.executorName = name
		return nil
	}
}

func WithRunnerName(name string) Option {
	return func(o *options) error {
		o.runnerName = name
		return nil
	}
}

func WithStartedAt(t time.Time) Option {
	return func(o *options) error {
		o.startedAt = t
		return nil
	}
}
