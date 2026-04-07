package concrete

import (
	"context"
	"encoding/json"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/step-runner/pkg/runner"
	"gitlab.com/gitlab-org/step-runner/proto"
)

func Spec() *proto.Spec {
	return &proto.Spec{
		Spec: &proto.Spec_Content{
			Inputs: map[string]*proto.Spec_Content_Input{
				"config": {
					Type: proto.ValueType_string,
				},
			},
		},
	}
}

func Run(ctx context.Context, builtinCtx runner.BuiltinContext) error {
	configRaw, err := builtinCtx.GetInput("config", runner.KindString)
	if err != nil {
		return err
	}

	var config run.Config
	if err := json.Unmarshal([]byte(configRaw.GetStringValue()), &config); err != nil {
		return err
	}

	runner, err := run.New(config, builtinCtx)
	if err != nil {
		return err
	}

	return runner.Run(ctx)
}
