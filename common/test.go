package common

func Int64Ptr(v int64) *int64 {
	return &v
}

type TestRunnerConfig struct {
	RunnerConfig *RunnerConfig
}

func NewTestRunnerConfig() *TestRunnerConfig {
	return &TestRunnerConfig{
		RunnerConfig: &RunnerConfig{},
	}
}

func (c *TestRunnerConfig) WithAutoscalerConfig(ac *AutoscalerConfig) *TestRunnerConfig {
	c.RunnerConfig.Autoscaler = ac
	return c
}

func (c *TestRunnerConfig) WithToken(token string) *TestRunnerConfig {
	c.RunnerConfig.RunnerCredentials.Token = token
	return c
}

type TestAutoscalerConfig struct {
	AutoscalerConfig *AutoscalerConfig
}

func NewTestAutoscalerConfig() *TestAutoscalerConfig {
	return &TestAutoscalerConfig{
		AutoscalerConfig: &AutoscalerConfig{},
	}
}

func (c *TestAutoscalerConfig) WithPolicies(policies ...AutoscalerPolicyConfig) *TestAutoscalerConfig {
	c.AutoscalerConfig.Policy = policies
	return c
}
