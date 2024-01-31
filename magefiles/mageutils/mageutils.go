package mageutils

import (
	"os"
	"sync"
)

// GetEnv allows us to mock os.Getenv in tests
// please don't override this outside of tests
var GetEnv = os.Getenv

func Env(env string) string {
	return GetEnv(env)
}

func EnvOrDefault(env, def string) string {
	return EnvFallbackOrDefault(env, "", def)
}

func EnvFallbackOrDefault(env, fallback, def string) string {
	val := Env(env)
	if val != "" {
		return val
	}
	if fallback != "" {
		val = Env(fallback)
		if val != "" {
			return val
		}
	}

	return def
}

type Once[T any] struct {
	val T

	o sync.Once
}

func (o *Once[T]) Do(fn func() (T, error)) T {
	o.o.Do(func() {
		var err error
		o.val, err = fn()

		if err != nil {
			panic(err)
		}
	})

	return o.val
}
