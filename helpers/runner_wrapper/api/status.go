package api

import (
	pb "gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api/proto"
)

type Status int64

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusInShutdown
	StatusStopped
)

func (s Status) String() string {
	statusesMap := map[Status]string{
		StatusUnknown:    "unknown",
		StatusRunning:    "running",
		StatusInShutdown: "in_shutdown",
		StatusStopped:    "stopped",
	}

	statusStr, ok := statusesMap[s]
	if !ok {
		return "unknown"
	}

	return statusStr
}

type statusMap map[Status]pb.Status

var (
	Statuses = statusMap{
		StatusUnknown:    pb.Status_unknown,
		StatusRunning:    pb.Status_running,
		StatusInShutdown: pb.Status_in_shutdown,
		StatusStopped:    pb.Status_stopped,
	}
)

func (s statusMap) Map(status Status) pb.Status {
	pbStatus, ok := s[status]
	if !ok {
		pbStatus = pb.Status_unknown
	}

	return pbStatus
}

func (s statusMap) Reverse(status pb.Status) Status {
	for aS, pbS := range s {
		if pbS == status {
			return aS
		}
	}

	return StatusUnknown
}
