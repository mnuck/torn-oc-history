package config

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// SetupLogging configures zerolog based on the config
func (c *Config) SetupLogging() {
	// Configure log output format
	if c.IsProduction() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(os.Stderr)
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	// Set log level
	level := c.getZerologLevel()
	zerolog.SetGlobalLevel(level)

	log.Debug().Msgf("Logging initialized with level: %s", c.LogLevel)
}

func (c *Config) getZerologLevel() zerolog.Level {
	levelStr := strings.ToLower(c.LogLevel)
	switch levelStr {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	default:
		log.Warn().Msgf("Unknown log level '%s', defaulting to info.", levelStr)
		return zerolog.InfoLevel
	}
}