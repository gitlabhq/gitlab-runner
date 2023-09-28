//go:build mage

package main

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/magefile/mage/sh"
)

type mageConfig struct {
	// Concurrency controls the amount of concurrent operations that can be performed by any given target.
	// For example if pushing packages, how many can be pushed concurrently in separate goroutines.
	Concurrency int
	// DryRun if supplied and if the target allows will not perform any destructive or creative actions but just log instead
	DryRun bool
}

var config mageConfig

func init() {
	envconfig.MustProcess("RUNNER_MAGE", &config)

	if config.Concurrency < 1 {
		config.Concurrency = 1
	}
}

// Generate runs go generate for all files in the magefiles directory
func Generate() error {
	return sh.RunV("go", "generate", "-tags", "mage", "./magefiles")
}
