//go:build !integration

package common

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/usage_log/logrotate"
)

func TestUsageLogRecordFrom_StoreDoesNotMutateRunnerLabels(t *testing.T) {
	runner := newRunnerConfigWithLabels()
	record, err := UsageLogRecordFrom(runner, &Build{})
	require.NoError(t, err)

	w := newUsageLogWriter(t)
	require.NoError(t, w.Store(record))

	assert.Equal(t, Labels{
		"global-label": "global-value",
		"runner-label": "runner-value",
	}, runner.ComputedLabels())
}

func TestUsageLogRecordFrom_StoreConcurrentWithMarshal(t *testing.T) {
	runner := newRunnerConfigWithLabels()
	record, err := UsageLogRecordFrom(runner, &Build{})
	require.NoError(t, err)

	w := newUsageLogWriter(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			_, _ = json.Marshal(Info{Labels: runner.ComputedLabels()})
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			_ = w.Store(record)
		}
	}()
	wg.Wait()
}

func newRunnerConfigWithLabels() *RunnerConfig {
	runner := &RunnerConfig{
		RunnerSettings: RunnerSettings{
			Labels: Labels{"runner-label": "runner-value"},
		},
	}
	runner.ComputeLabels(Labels{"global-label": "global-value"})
	return runner
}

func newUsageLogWriter(t *testing.T) *logrotate.Writer {
	w := logrotate.New(
		logrotate.WithLogDirectory(t.TempDir()),
		logrotate.WithLabels(map[string]string{"writer-label": "writer-value"}),
	)
	t.Cleanup(func() {
		require.NoError(t, w.Close())
	})
	return w
}
