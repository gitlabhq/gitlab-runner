package usage_log

import (
	"time"
)

type Record struct {
	Timestamp time.Time         `json:"timestamp"`
	Runner    Runner            `json:"runner"`
	Job       Job               `json:"job"`
	Labels    map[string]string `json:"labels"`
}

type Runner struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SystemID string `json:"system_id"`
}

type Job struct {
	URL             string    `json:"url"`
	DurationSeconds float64   `json:"duration_seconds"`
	Status          string    `json:"status"`
	FailureReason   string    `json:"failure_reason"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
}
