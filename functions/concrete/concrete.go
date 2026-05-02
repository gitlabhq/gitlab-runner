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

	cancelCtx, cancel := builtinCtx.ListenCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done(): // no-op, the main context was cancelled OR the Close API was called.
		case <-cancelCtx.Done(): // the cancel API was called.
			runner.Cancel()
		}
	}()

	return runner.Run(ctx)
}
