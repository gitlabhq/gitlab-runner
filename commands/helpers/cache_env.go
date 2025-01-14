package helpers

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func loadEnvFile(filename string) error {
	if filename == "" {
		return nil
	}

	env, err := godotenv.Read(filename)
	if err != nil {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	return nil
}
