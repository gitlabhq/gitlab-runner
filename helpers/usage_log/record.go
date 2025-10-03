package usage_log

import (
	"time"
)

type Record struct {
	UUID      string            `json:"uuid"`
	Timestamp time.Time         `json:"timestamp"`
	Runner    Runner            `json:"runner"`
	Job       Job               `json:"job"`
	Labels    map[string]string `json:"labels"`
}

type Runner struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SystemID string `json:"system_id"`
	Executor string `json:"executor"`
}

type Job struct {
	URL             string    `json:"url"`
	DurationSeconds float64   `json:"duration_seconds"`
	Status          string    `json:"status"`
	FailureReason   string    `json:"failure_reason"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`

	Project       Project      `json:"project"`
	Namespace     Namespace    `json:"namespace"`
	RootNamespace Namespace    `json:"root_namespace"`
	Organization  Organization `json:"organization"`
	Instance      Instance     `json:"instance"`
	User          User         `json:"user"`
	ScopedUser    User         `json:"scoped_user"`
}

type Project struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

type Namespace struct {
	ID int64 `json:"id"`
}

type Organization struct {
	ID int64 `json:"id"`
}

type Instance struct {
	ID       string `json:"id"`
	UniqueID string `json:"unique_id"`
}

type User struct {
	ID int64 `json:"id"`
}
