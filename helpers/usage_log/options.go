package usage_log

type options struct {
	Labels map[string]string
}

type Option func(*options)

func setupOptions(o ...Option) options {
	opts := options{
		Labels: make(map[string]string),
	}

	for _, opt := range o {
		opt(&opts)
	}

	return opts
}

func WithLabels(labels map[string]string) Option {
	return func(o *options) {
		o.Labels = labels
	}
}
