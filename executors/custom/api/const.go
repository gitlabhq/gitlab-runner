package api

const (
	// The name of the variable used to pass the value of Build failure exit code
	// that should be returned from Custom executor driver
	BuildFailureExitCodeVariable = "BUILD_FAILURE_EXIT_CODE"

	// The name of the variable used to pass the value of System failure exit code
	// that should be returned from Custom executor driver
	SystemFailureExitCodeVariable = "SYSTEM_FAILURE_EXIT_CODE"

	// The name of the variable used to pass the value of path to the file that
	// contains JSON encoded content of job API received from GitLab's API
	JobResponseFileVariable = "JOB_RESPONSE_FILE"
)
