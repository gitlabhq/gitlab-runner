package configfile

import "gitlab.com/gitlab-org/gitlab-runner/common"

type options struct {
	AccessCollector bool
	Config          *common.Config
	SystemID        string
}

type Option func(*options)

func WithAccessCollector() Option {
	return func(o *options) {
		o.AccessCollector = true
	}
}

func WithExistingConfig(config *common.Config) Option {
	return func(o *options) {
		o.Config = config
	}
}

func WithSystemID(systemID string) Option {
	return func(o *options) {
		o.SystemID = systemID
	}
}

type loadOptions struct {
	Mutate []func(cfg *common.Config) error
}

type LoadOption func(*loadOptions)

func WithMutateOnLoad(fn func(cfg *common.Config) error) LoadOption {
	return func(o *loadOptions) {
		o.Mutate = append(o.Mutate, fn)
	}
}
