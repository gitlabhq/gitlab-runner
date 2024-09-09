//go:build !integration

package common

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testJobState struct {
	// the value is unexported, unexported values can't be encoded/decoded
	// that's how we'll know the custom encoding works
	value int

	marshalErr error
}

type encodedJobState struct {
	Value int

	unmarshalErr error
}

func (s *testJobState) ToEncoded() (any, error) {
	return &encodedJobState{Value: s.value}, s.marshalErr
}

func (from *encodedJobState) FromEncoded() (any, error) {
	return &testJobState{value: from.Value}, from.unmarshalErr
}

func init() {
	RegisterJobState(&encodedJobState{})
}

func TestJobRuntimeState_MarshalInto(t *testing.T) {
	tests := map[string]struct {
		state    func() *JobRuntimeState
		expected func(state *JobRuntimeState) (*EncodedJobRuntimeState, error)
	}{
		"NewJobRuntimeState": {
			state: NewJobRuntimeState,
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return &EncodedJobRuntimeState{
					HealthCheckAt: state.GetHealthCheckAt(),
					StartedAt:     state.GetStartedAt(),
				}, nil
			},
		},
		"nil": {
			state: func() *JobRuntimeState {
				return nil
			},
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return nil, nil
			},
		},
		"executorStateMetadata is nil": {
			state: func() *JobRuntimeState {
				s := NewJobRuntimeState()
				s.executorStateMetadata = nil
				return s
			},
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return &EncodedJobRuntimeState{
					HealthCheckAt: state.GetHealthCheckAt(),
					StartedAt:     state.GetStartedAt(),
				}, nil
			},
		},
		"executorStateMetadata is not a JobMarshalInto": {
			state: func() *JobRuntimeState {
				s := NewJobRuntimeState()
				s.executorStateMetadata = "not a JobMarshalInto"
				return s
			},
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return &EncodedJobRuntimeState{
					HealthCheckAt:         state.GetHealthCheckAt(),
					StartedAt:             state.GetStartedAt(),
					ExecutorStateMetadata: "not a JobMarshalInto",
				}, nil
			},
		},
		"all fields set": {
			state: func() *JobRuntimeState {
				s := NewJobRuntimeState()
				s.retries = 2
				s.buildState = BuildRunRuntimeRunning
				s.stage = "stage"
				s.sentTrace = 33
				s.executorStateMetadata = &testJobState{value: 5}
				return s
			},
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return &EncodedJobRuntimeState{
					Retries:       2,
					BuildState:    BuildRunRuntimeRunning,
					Stage:         "stage",
					SentTrace:     33,
					HealthCheckAt: state.GetHealthCheckAt(),
					StartedAt:     state.GetStartedAt(),
					ExecutorStateMetadata: &encodedJobState{
						Value: 5,
					},
				}, nil
			},
		},
		"executorStateMetadata marshaling returns error": {
			state: func() *JobRuntimeState {
				s := NewJobRuntimeState()
				s.executorStateMetadata = &testJobState{value: 5, marshalErr: errors.New("err")}
				return s
			},
			expected: func(state *JobRuntimeState) (*EncodedJobRuntimeState, error) {
				return nil, errors.New("err")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			expected, expectedErr := tt.expected(tt.state())
			actual, err := tt.state().ToEncoded()
			if expectedErr != nil {
				require.EqualError(t, err, expectedErr.Error())
				return
			}
			if expected == nil {
				require.Nil(t, actual)
				return
			}

			assert.Equal(t, expected.Retries, actual.Retries)
			assert.Equal(t, expected.BuildState, actual.BuildState)
			assert.Equal(t, expected.Stage, actual.Stage)
			assert.Equal(t, expected.SentTrace, actual.SentTrace)
			assert.Equal(t, expected.ExecutorStateMetadata, actual.ExecutorStateMetadata)
			// comparing time in go is fun
			assert.WithinDuration(t, expected.HealthCheckAt, actual.HealthCheckAt, time.Millisecond)
			assert.WithinDuration(t, expected.StartedAt, actual.StartedAt, time.Millisecond)
		})
	}
}

func TestJobRuntimeState_UnmarshalInto(t *testing.T) {
	tests := map[string]struct {
		encodedState func() *EncodedJobRuntimeState
		expected     func() (*JobRuntimeState, error)
	}{
		"nil": {
			encodedState: func() *EncodedJobRuntimeState {
				return nil
			},
			expected: func() (*JobRuntimeState, error) {
				return nil, nil
			},
		},
		"executorStateMetadata is nil": {
			encodedState: func() *EncodedJobRuntimeState {
				return &EncodedJobRuntimeState{}
			},
			expected: func() (*JobRuntimeState, error) {
				return &JobRuntimeState{}, nil
			},
		},
		"executorStateMetadata is not a JobUnmarshalInto": {
			encodedState: func() *EncodedJobRuntimeState {
				return &EncodedJobRuntimeState{
					ExecutorStateMetadata: "not a JobUnmarshalInto",
				}
			},
			expected: func() (*JobRuntimeState, error) {
				state := &JobRuntimeState{}
				state.executorStateMetadata = "not a JobUnmarshalInto"
				return state, nil
			},
		},
		"all fields set": {
			encodedState: func() *EncodedJobRuntimeState {
				return &EncodedJobRuntimeState{
					Retries:       2,
					BuildState:    BuildRunRuntimeRunning,
					Stage:         "stage",
					SentTrace:     33,
					HealthCheckAt: time.Now(),
					StartedAt:     time.Now(),
					ExecutorStateMetadata: &encodedJobState{
						Value: 5,
					},
				}
			},
			expected: func() (*JobRuntimeState, error) {
				state := NewJobRuntimeState()
				state.retries = 2
				state.buildState = BuildRunRuntimeRunning
				state.stage = "stage"
				state.sentTrace = 33
				state.healthCheckAt = time.Now()
				state.startedAt = time.Now()
				state.executorStateMetadata = &testJobState{value: 5}
				return state, nil
			},
		},
		"executorStateMetadata unmarshaling returns error": {
			encodedState: func() *EncodedJobRuntimeState {
				return &EncodedJobRuntimeState{
					ExecutorStateMetadata: &encodedJobState{unmarshalErr: errors.New("err")},
				}
			},
			expected: func() (*JobRuntimeState, error) {
				return nil, errors.New("err")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			expected, expectedErr := tt.expected()
			actual, err := tt.encodedState().FromEncoded()
			if expectedErr != nil {
				require.EqualError(t, err, expectedErr.Error())
				return
			}
			if expected == nil {
				require.Nil(t, actual)
				return
			}

			assert.Equal(t, expected.retries, actual.retries)
			assert.Equal(t, expected.buildState, actual.buildState)
			assert.Equal(t, expected.stage, actual.stage)
			assert.Equal(t, expected.sentTrace, actual.sentTrace)
			assert.Equal(t, expected.executorStateMetadata, actual.executorStateMetadata)
			// comparing time in go is fun
			assert.WithinDuration(t, expected.healthCheckAt, actual.healthCheckAt, time.Millisecond)
			assert.WithinDuration(t, expected.startedAt, actual.startedAt, time.Millisecond)
		})
	}
}
