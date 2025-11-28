package common

import "testing"

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

// mockLightJobTrace is wrapper around common.MockJobTrace.
// The only difference is the Write method which does
// nothing but return the length of data it receives.
//
// This is done as mockery generated mocks maintain
// and internal state to make assertion but for this
// particular test it leads to excessive use of memory
// sometimes more than 50GB as the build test generates
// a lot of logs and processes them.
//
// This leads to OOM kills with Kubernetes runners.
//
// Note: When using mockLightJobTrace assert on Write method
// will not work.
type mockLightJobTrace struct {
	*MockJobTrace
}

func NewMockLightJobTrace(t *testing.T) *mockLightJobTrace {
	return &mockLightJobTrace{
		MockJobTrace: NewMockJobTrace(t),
	}
}

func (l *mockLightJobTrace) Write(p []byte) (int, error) {
	return len(p), nil
}
