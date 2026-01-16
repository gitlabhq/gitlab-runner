package steps

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
	"gopkg.in/yaml.v2"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func NewRequest(build *common.Build) (*client.RunRequest, error) {
	steps, err := addStepsPreamble(build.Job.Run)
	if err != nil {
		return nil, fmt.Errorf("parsing step request: %w", err)
	}
	return &client.RunRequest{
		Id:        strconv.FormatInt(build.ID, 10),
		WorkDir:   build.FullProjectDir(),
		BuildDir:  build.FullProjectDir(),
		Env:       map[string]string{},
		Steps:     steps,
		Variables: addVariables(build.GetAllVariables()),
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
func addStepsPreamble(jsonSteps string) (string, error) {
	stepSchema := &schema.Step{}

	err := json.Unmarshal([]byte(jsonSteps), &stepSchema.Run)
	if err != nil {
		return "", fmt.Errorf("unmarshalling steps %q: %w", jsonSteps, err)
	}
	yamlSteps, err := yaml.Marshal(stepSchema)
	if err != nil {
		return "", fmt.Errorf("marshalling steps: %w", err)
	}

	return stepsPreamble + string(yamlSteps), nil
}
