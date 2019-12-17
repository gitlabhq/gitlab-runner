package referees

type MockExecutor struct{}

func (me *MockExecutor) GetMetricsLabelName() string {
	return "name"
}

func (me *MockExecutor) GetMetricsLabelValue() string {
	return "value"
}
