package autoscaler

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//nolint:nestif
func withInit(ctx context.Context, config *common.RunnerConfig, nc api.Client, call func() error) error {
	// Try the call
	err := call()
	if err == nil {
		return nil
	}

	// Error making the call
	if !errors.Is(err, api.ErrNotInitialized) {
		return err
	}

	// Lazy initialization
	nestingInitCfg, err := config.Autoscaler.VMIsolation.NestingConfig.JSON()
	if err != nil {
		return fmt.Errorf("converting nesting init config to json: %w", err)
	}

	err = nc.Init(ctx, nestingInitCfg)
	// Error initializing
	if err != nil && !errors.Is(err, api.ErrAlreadyInitialized) {
		return fmt.Errorf("initializing nesting: %w", err)
	}

	// Try the call again
	return call()
}
