package referees

type mockMetricsExecutor struct{}

func (m *mockMetricsExecutor) GetMetricsLabelName() string {
	return "name"
}

func (m *mockMetricsExecutor) GetMetricsLabelValue() string {
	return "value"
}
