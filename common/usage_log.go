package common

import (
	"gitlab.com/gitlab-org/gitlab-runner/helpers/usage_log"
)

func UsageLogRecordFrom(runner *RunnerConfig, build *Build) usage_log.Record {
	record := usage_log.Record{
		Runner: usage_log.Runner{
			ID:       runner.ShortDescription(),
			Name:     runner.Name,
			SystemID: runner.GetSystemID(),
			Executor: runner.Executor,
		},
		Job: usage_log.Job{
			URL:             build.JobURL(),
			DurationSeconds: build.FinalDuration().Seconds(),
			Status:          build.CurrentState().String(),
			FailureReason:   build.FailureReason().String(),
			StartedAt:       build.StartedAt().UTC(),
			FinishedAt:      build.FinishedAt().UTC(),
			Project: usage_log.Project{
				ID:       build.JobInfo.ProjectID,
				Name:     build.JobInfo.ProjectName,
				FullPath: build.JobInfo.ProjectFullPath,
			},
			Namespace: usage_log.Namespace{
				ID: build.JobInfo.NamespaceID,
			},
			RootNamespace: usage_log.Namespace{
				ID: build.JobInfo.RootNamespaceID,
			},
			Organization: usage_log.Organization{
				ID: build.JobInfo.OrganizationID,
			},
			Instance: usage_log.Instance{
				ID:       build.JobInfo.InstanceID,
				UniqueID: build.JobInfo.InstanceUUID,
			},
			User: usage_log.User{
				ID: build.JobInfo.UserID,
			},
		},
		Labels: runner.ComputedLabels(),
	}

	if build.JobInfo.ScopedUserID != nil {
		record.Job.ScopedUser.ID = *build.JobInfo.ScopedUserID
	}

	return record
}
