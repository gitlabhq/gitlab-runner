package steps

import (
	"errors"
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
	"go.yaml.in/yaml/v3"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

// ErrNoStepRunnerButOkay is returned from Connect() when we cannot establish
// a connection to the step-runner, but execution can continue normally.
//
// This occurs with the Docker executor when the container executes code directly
// instead of starting the step-runner process. For example, when the job script
// runs as the image ENTRYPOINT and the container terminates after execution.
//
// See: https://docs.gitlab.com/runner/executors/docker/#job-script-as-entrypoint
var ErrNoStepRunnerButOkay = errors.New("no step runner but okay")

func NewRequest(jobInfo JobInfo, steps []schema.Step) (*client.RunRequest, error) {
	preambleSteps, err := addStepsPreamble(steps)
	if err != nil {
		return nil, fmt.Errorf("parsing step request: %w", err)
	}

	return &client.RunRequest{
		Id:        strconv.FormatInt(jobInfo.ID, 10),
		Timeout:   &jobInfo.Timeout,
		WorkDir:   jobInfo.ProjectDir,
		BuildDir:  jobInfo.ProjectDir,
		Env:       map[string]string{},
		Steps:     preambleSteps,
		Variables: addVariables(jobInfo.Variables),
	}, nil
}

var variablesToOmit = map[string]bool{
	"STEPS":               true,
	"FF_USE_NATIVE_STEPS": true,
}

func addVariables(vars spec.Variables) []client.Variable {
	result := make([]client.Variable, 0, len(vars))
	for _, v := range vars {
		if variablesToOmit[v.Key] {
			continue
		}

		result = append(result, client.Variable{
			Key:    v.Key,
			Value:  v.Value,
			File:   v.File,
			Masked: v.Masked,
		})
	}
	return result
}

const stepsPreamble = "{}\n---\n"

// When using the run CI keyword, steps are written in yaml, parsed to json, validated, and finally sent over the wire
// (as json) to the runner. However the step-runner expects steps as yaml :-( Plus we have to add this here preamble.
func addStepsPreamble(steps []schema.Step) (string, error) {
	stepSchema := &schema.Step{}
	stepSchema.Run = steps

	yamlSteps, err := yaml.Marshal(stepSchema)
	if err != nil {
		return "", fmt.Errorf("marshalling steps: %w", err)
	}

	return stepsPreamble + string(yamlSteps), nil
}
