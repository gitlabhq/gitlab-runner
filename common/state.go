package common

import (
	"encoding/gob"
	"reflect"
	"sync"
	"time"
)

type StatefulExecutor interface {
	Resume(cmd ExecutorCommand) error
	GetState() any
	SetState(any) bool
}

// JobRuntimeState represents the state of a job that can be serialized to a store.
// All public fields must be accessed through their respective getters and setters.
type JobRuntimeState struct {
	mu sync.RWMutex

	resumedFromStage BuildStage

	retries               int
	buildState            BuildRuntimeState
	stage                 BuildStage
	healthCheckAt         time.Time
	startedAt             time.Time
	sentTrace             int64
	executorStateMetadata any
}

var _ JobStateToEncoded[*EncodedJobRuntimeState] = (*JobRuntimeState)(nil)

type EncodedJobRuntimeState struct {
	Retries               int
	BuildState            BuildRuntimeState
	Stage                 BuildStage
	HealthCheckAt         time.Time
	StartedAt             time.Time
	SentTrace             int64
	ExecutorStateMetadata any
}

var _ JobStateFromEncoded[*JobRuntimeState] = (*EncodedJobRuntimeState)(nil)

var registeredJobStates sync.Map

func RegisterJobState(state any) {
	_, loaded := registeredJobStates.LoadOrStore(reflect.TypeOf(state), nil)
	if loaded {
		return
	}

	// Register the state type, so it can be encoded/decoded
	// by the gob package. Since we currently only support gob
	// this function abstracts gob away from executors.
	gob.Register(state)
}

func init() {
	RegisterJobState(&EncodedJobRuntimeState{})
}

func (s *JobRuntimeState) ToEncoded() (*EncodedJobRuntimeState, error) {
	if s == nil {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	executorStateMetadata := s.executorStateMetadata
	if state, ok := executorStateMetadata.(JobStateToEncoded[any]); ok {
		var err error
		executorStateMetadata, err = state.ToEncoded()
		if err != nil {
			return nil, err
		}
	}

	return &EncodedJobRuntimeState{
		Retries:               s.retries,
		BuildState:            s.buildState,
		Stage:                 s.stage,
		HealthCheckAt:         s.healthCheckAt,
		StartedAt:             s.startedAt,
		SentTrace:             s.sentTrace,
		ExecutorStateMetadata: executorStateMetadata,
	}, nil
}

func (from *EncodedJobRuntimeState) FromEncoded() (*JobRuntimeState, error) {
	if from == nil {
		return nil, nil
	}

	executorStateMetadata := from.ExecutorStateMetadata
	if state, ok := executorStateMetadata.(JobStateFromEncoded[any]); ok {
		var err error
		executorStateMetadata, err = state.FromEncoded()
		if err != nil {
			return nil, err
		}
	}

	return &JobRuntimeState{
		retries:               from.Retries,
		buildState:            from.BuildState,
		stage:                 from.Stage,
		healthCheckAt:         from.HealthCheckAt,
		startedAt:             from.StartedAt,
		sentTrace:             from.SentTrace,
		executorStateMetadata: executorStateMetadata,
	}, nil
}

func NewJobRuntimeState() *JobRuntimeState {
	now := time.Now()
	return &JobRuntimeState{
		healthCheckAt: now,
		startedAt:     now,
	}
}

func (s *JobRuntimeState) SetBuildState(state BuildRuntimeState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If the state is already set we don't want to go back to pending
	if state == BuildRunStatePending && s.buildState != "" {
		return
	}

	s.buildState = state
}

func (s *JobRuntimeState) GetBuildState() BuildRuntimeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.buildState
}

func (s *JobRuntimeState) SetStage(stage BuildStage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stage = stage
}

func (s *JobRuntimeState) GetStage() BuildStage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stage
}

func (s *JobRuntimeState) GetResumedFromStage() BuildStage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resumedFromStage
}

func (s *JobRuntimeState) UnsetResumedFromStage() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resumedFromStage = ""
}

func (s *JobRuntimeState) UpdateHealth() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.healthCheckAt = time.Now()
}

func (s *JobRuntimeState) GetHealthCheckAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.healthCheckAt
}

func (s *JobRuntimeState) IsResumed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retries > 0 && s.resumedFromStage != ""
}

func (s *JobRuntimeState) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.retries++
	s.resumedFromStage = s.stage
}

func (s *JobRuntimeState) GetRetries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.retries
}

func (s *JobRuntimeState) SetSentTrace(offset int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sentTrace = offset
}

func (s *JobRuntimeState) GetSentTrace() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sentTrace
}

func (s *JobRuntimeState) GetStartedAt() time.Time {
	return s.startedAt
}

func (s *JobRuntimeState) SetExecutorState(state any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.executorStateMetadata = state
}

func (s *JobRuntimeState) GetExecutorState() any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.executorStateMetadata
}
