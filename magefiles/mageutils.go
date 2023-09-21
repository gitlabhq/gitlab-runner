//go:build mage

package main

import (
	"os"
)

func envFallbackOrDefault(env, fallback, def string) string {
	val := os.Getenv(env)
	if val != "" {
		return val
	}
	val = os.Getenv(fallback)
	if val != "" {
		return val
	}

	return def
}
