package main

import (
	"log/slog"
	"os"

	"torn-oc-history/internal/env"
	"torn-oc-history/internal/log"
)

// setupEnvironment loads .env file and configures logging.
func setupEnvironment() {
	// Load .env file if it exists
	err := env.Load(".env")

	// Configure logging
	log.Setup()

	// wait until now to report on the .env file so we have the chance to set up logging first
	if err == nil {
		slog.Debug("Loaded environment variables from .env file.")
	} else {
		slog.Debug("No .env file found or error loading .env file; proceeding with existing environment variables.")
	}
}

// getRequiredEnv fetches a required environment variable or exits if not set.
func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error(key+" environment variable is required.")
		os.Exit(1)
	}
	return value
}

// getEnvWithDefault fetches an environment variable with a default fallback.
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
