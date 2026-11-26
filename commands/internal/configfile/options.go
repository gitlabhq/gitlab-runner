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
	Mutate           []func(cfg *common.Config) error
	SkipSystemIDSave bool
}

type LoadOption func(*loadOptions)

func WithMutateOnLoad(fn func(cfg *common.Config) error) LoadOption {
	return func(o *loadOptions) {
		o.Mutate = append(o.Mutate, fn)
	}
}

// WithoutSavingSystemID keeps a newly generated system ID in memory only, for
// commands that must not leave a .runner_system_id file behind. Commands that
// send the system ID to GitLab to create or verify a runner manager (run,
// run-single, register, verify) must not use it, or GitLab would track a
// manager whose ID changes on every start.
func WithoutSavingSystemID() LoadOption {
	return func(o *loadOptions) {
		o.SkipSystemIDSave = true
	}
}
