package common

import (
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

	Retries               int
	BuildState            BuildRuntimeState
	Stage                 BuildStage
	HealthCheckAt         time.Time
	StartedAt             time.Time
	SentTrace             int64
	ExecutorStateMetadata any
}

func NewJobRuntimeState() *JobRuntimeState {
	now := time.Now()
	return &JobRuntimeState{
		HealthCheckAt: now,
		StartedAt:     now,
	}
}

func (s *JobRuntimeState) SetBuildState(state BuildRuntimeState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If the state is already set we don't want to go back to pending
	if state == BuildRunStatePending && s.BuildState != "" {
		return
	}

	s.BuildState = state
}

func (s *JobRuntimeState) GetBuildState() BuildRuntimeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.BuildState
}

func (s *JobRuntimeState) SetStage(stage BuildStage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Stage = stage
}

func (s *JobRuntimeState) GetStage() BuildStage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Stage
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

	s.HealthCheckAt = time.Now()
}

func (s *JobRuntimeState) GetHealthCheckAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.HealthCheckAt
}

func (s *JobRuntimeState) IsResumed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Retries > 0 && s.resumedFromStage != ""
}

func (s *JobRuntimeState) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Retries++
	s.resumedFromStage = s.Stage
}

func (s *JobRuntimeState) GetRetries() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Retries
}

func (s *JobRuntimeState) SetSentTrace(offset int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SentTrace = offset
}

func (s *JobRuntimeState) GetSentTrace() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.SentTrace
}

func (s *JobRuntimeState) GetStartedAt() time.Time {
	return s.StartedAt
}

func (s *JobRuntimeState) SetExecutorState(state any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExecutorStateMetadata = state
}

func (s *JobRuntimeState) GetExecutorState() any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ExecutorStateMetadata
}
