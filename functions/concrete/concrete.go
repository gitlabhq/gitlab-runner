// Package concrete is the step-runner builtin counterpart to the abstract
// shell in shells/abstract.go. Stage logic is largely a direct port; this
// note records intentional divergences so the next reader does not chase
// them as gaps.
//
//   - FF_CLEAN_UP_FAILED_CACHE_EXTRACT (issue #36988, MR !4565):
//     The abstract shell removes the user-declared cache paths after a
//     failed extraction to recover from a partially-extracted directory
//     left behind by an OOM-killed cache-extractor process. That
//     originated with the Kubernetes executor running cache-extractor in
//     a separate (memory-constrained) helper container, where SIGKILL
//     could leave orphan files for the next job to inherit.
//
//     Concrete runs cache-extractor inside the build environment that
//     step-runner is itself executing in, so a SIGKILL of the extractor
//     almost certainly takes the surrounding context with it; the
//     orphaned-partial-extract failure mode the FF was protecting
//     against does not have a clear analog here. The behaviour is also
//     wrong: it removes pre-existing files in the cache path (e.g.
//     files dropped by git clone or a prior step) along with the
//     partial extract.
//
//     We therefore do not implement FF_CLEAN_UP_FAILED_CACHE_EXTRACT
//     here. If the failure class re-emerges in concrete it should be
//     handled more precisely than rm -rf of user-declared paths (e.g.
//     extract to staging then promote, or have the extractor track and
//     remove only what it wrote).
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
