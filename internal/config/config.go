package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	// API Configuration
	TornAPIKey string `mapstructure:"torn_api_key"`

	// Google Sheets Configuration
	SpreadsheetID string `mapstructure:"spreadsheet_id"`
	CredentialsFile string `mapstructure:"credentials_file"`

	// Application Configuration
	Output string `mapstructure:"output"`
	All bool `mapstructure:"all"`
	Both bool `mapstructure:"both"`
	RangeNOC string `mapstructure:"range_noc"`
	RangeAll string `mapstructure:"range_all"`
	Interval time.Duration `mapstructure:"interval"`

	// Logging Configuration
	Environment string `mapstructure:"environment"`
	LogLevel string `mapstructure:"log_level"`
}

func New() (*Config, error) {
	// Load .env file if it exists (like the old setupEnvironment function)
	err := godotenv.Load()
	var envFileLoaded bool
	if err == nil {
		envFileLoaded = true
	}

	v := viper.New()

	// Set configuration name and paths
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.torn-oc-history")
	v.AddConfigPath("/etc/torn-oc-history")

	// Set defaults
	setDefaults(v)

	// Read from environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("TORN")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// Bind environment variables explicitly
	bindEnvVars(v)

	// Try to read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Parse command line flags
	setupFlags()
	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}

	// Parse flags (if not already parsed)
	if !pflag.CommandLine.Parsed() {
		pflag.Parse()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply log level defaults based on environment (like the old setup.go)
	cfg.applyLogLevelDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Setup logging early so we can log about the .env file
	cfg.SetupLogging()

	// Log about .env file loading (like the old setup.go)
	if envFileLoaded {
		log.Debug().Msg("Loaded environment variables from .env file.")
	} else {
		log.Debug().Msg("No .env file found or error loading .env file; proceeding with existing environment variables.")
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Application defaults
	v.SetDefault("output", "stdout")
	v.SetDefault("all", false)
	v.SetDefault("both", false)
	v.SetDefault("range_noc", "History!A1")
	v.SetDefault("range_all", "HistoryAll!A1")
	v.SetDefault("interval", time.Duration(0))

	// Google Sheets defaults
	v.SetDefault("credentials_file", "credentials.json")

	// Logging defaults
	v.SetDefault("environment", "development")
	v.SetDefault("log_level", "info")
}

func bindEnvVars(v *viper.Viper) {
	envVars := []string{
		"torn_api_key",
		"spreadsheet_id",
		"credentials_file",
		"output",
		"all",
		"both",
		"range_noc",
		"range_all",
		"interval",
		"environment",
		"log_level",
	}

	for _, env := range envVars {
		v.BindEnv(env)
	}

	// Bind legacy environment variable names
	v.BindEnv("torn_api_key", "TORN_API_KEY")
	v.BindEnv("spreadsheet_id", "SPREADSHEET_ID")
	v.BindEnv("environment", "ENV")
	v.BindEnv("log_level", "LOGLEVEL")
}

func setupFlags() {
	pflag.String("output", "stdout", "output destination: stdout or sheets")
	pflag.Bool("all", false, "Generate report for all faction members")
	pflag.Bool("both", false, "Generate both reports (all members and those not in OC)")
	pflag.String("range-noc", "History!A1", "Spreadsheet range for members not in OC")
	pflag.String("range-all", "HistoryAll!A1", "Spreadsheet range for all members report")
	pflag.Duration("interval", 0, "Repeat execution at this interval (e.g. 5m). 0 runs once")

	pflag.String("credentials-file", "credentials.json", "Path to Google Cloud service account credentials file")
	pflag.String("log-level", "info", "Log level (debug, info, warn, error, fatal, panic, disabled)")
	pflag.String("environment", "development", "Environment (development, production)")
}

func (c *Config) Validate() error {
	// Validate required fields
	if c.TornAPIKey == "" {
		return fmt.Errorf("TORN_API_KEY is required")
	}

	// Check if credentials file exists
	if _, err := os.Stat(c.CredentialsFile); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s", c.CredentialsFile)
	}

	// Validate output destination
	if c.Output != "stdout" && c.Output != "sheets" {
		return fmt.Errorf("output must be either 'stdout' or 'sheets', got: %s", c.Output)
	}

	// Validate that sheets output has required fields
	if c.Output == "sheets" && c.SpreadsheetID == "" {
		return fmt.Errorf("SPREADSHEET_ID is required when output is 'sheets'")
	}

	// Validate conflicting flags
	if c.All && c.Both {
		return fmt.Errorf("--all and --both cannot be used together")
	}

	// Validate log level
	validLevels := []string{"debug", "info", "warn", "warning", "error", "fatal", "panic", "disabled"}
	levelValid := false
	for _, level := range validLevels {
		if strings.ToLower(c.LogLevel) == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		return fmt.Errorf("invalid log level: %s (valid: %s)", c.LogLevel, strings.Join(validLevels, ", "))
	}

	return nil
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// applyLogLevelDefaults sets log level defaults based on environment when log level is empty
// This mirrors the behavior of the old setup.go
func (c *Config) applyLogLevelDefaults() {
	if c.LogLevel == "" {
		if c.IsProduction() {
			c.LogLevel = "warn"
		} else {
			c.LogLevel = "info"
		}
	}
}