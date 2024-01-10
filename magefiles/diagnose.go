package main

import (
	"context"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Diagnose mg.Namespace

// DiskUsage runs df -h every interval. The first printed output
// will be at the first timer tick
func (Diagnose) DiskUsage(ctx context.Context, interval string) error {
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(intervalDuration):
			if err := sh.RunV("df", "-h"); err != nil {
				return err
			}
		}
	}
}
