//go:build !integration

package test

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewHook(t *testing.T) {
	beforeCount := countHooks()

	_, cleanup := NewHook()
	afterCount := countHooks()

	cleanup()

	assert.True(t, afterCount > beforeCount)
	assert.Equal(t, beforeCount, countHooks())
}

func countHooks() int {
	count := 0
	for _, levels := range logrus.StandardLogger().Hooks {
		for range levels {
			count++
		}
	}

	return count
}
