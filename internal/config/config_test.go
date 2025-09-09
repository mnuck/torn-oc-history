package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				TornAPIKey:      "test-key",
				SpreadsheetID:   "test-id",
				CredentialsFile: "testdata/test-credentials.json",
				Output:          "stdout",
				LogLevel:        "info",
				Environment:     "development",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: Config{
				Output:   "stdout",
				LogLevel: "info",
			},
			wantErr: true,
			errMsg:  "TORN_API_KEY is required",
		},
		{
			name: "invalid output",
			config: Config{
				TornAPIKey:      "test-key",
				CredentialsFile: "testdata/test-credentials.json",
				Output:          "invalid",
				LogLevel:        "info",
			},
			wantErr: true,
			errMsg:  "output must be either 'stdout' or 'sheets', got: invalid",
		},
		{
			name: "sheets output without spreadsheet ID",
			config: Config{
				TornAPIKey:      "test-key",
				CredentialsFile: "testdata/test-credentials.json",
				Output:          "sheets",
				LogLevel:        "info",
			},
			wantErr: true,
			errMsg:  "SPREADSHEET_ID is required when output is 'sheets'",
		},
		{
			name: "both all and both flags set",
			config: Config{
				TornAPIKey:      "test-key",
				SpreadsheetID:   "test-id", 
				CredentialsFile: "testdata/test-credentials.json",
				Output:          "sheets",
				All:             true,
				Both:            true,
				LogLevel:        "info",
			},
			wantErr: true,
			errMsg:  "--all and --both cannot be used together",
		},
		{
			name: "invalid log level",
			config: Config{
				TornAPIKey:      "test-key",
				CredentialsFile: "testdata/test-credentials.json",
				Output:          "stdout",
				LogLevel:        "invalid",
			},
			wantErr: true,
			errMsg:  "invalid log level: invalid (valid: debug, info, warn, warning, error, fatal, panic, disabled)",
		},
	}

	// Create testdata directory and test credentials file
	os.MkdirAll("testdata", 0755)
	testCredentials := `{"type": "service_account", "project_id": "test"}`
	os.WriteFile("testdata/test-credentials.json", []byte(testCredentials), 0644)
	defer os.RemoveAll("testdata")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Config.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Config.Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Config.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		want        bool
	}{
		{
			name:        "production environment",
			environment: "production",
			want:        true,
		},
		{
			name:        "development environment",
			environment: "development", 
			want:        false,
		},
		{
			name:        "empty environment",
			environment: "",
			want:        false,
		},
		{
			name:        "other environment",
			environment: "staging",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Environment: tt.environment}
			if got := c.IsProduction(); got != tt.want {
				t.Errorf("Config.IsProduction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_ApplyLogLevelDefaults(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		logLevel    string
		want        string
	}{
		{
			name:        "production with empty log level",
			environment: "production",
			logLevel:    "",
			want:        "warn",
		},
		{
			name:        "development with empty log level",
			environment: "development",
			logLevel:    "",
			want:        "info",
		},
		{
			name:        "production with explicit log level",
			environment: "production",
			logLevel:    "debug",
			want:        "debug",
		},
		{
			name:        "development with explicit log level",
			environment: "development",
			logLevel:    "error",
			want:        "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Environment: tt.environment,
				LogLevel:    tt.logLevel,
			}
			c.applyLogLevelDefaults()
			if c.LogLevel != tt.want {
				t.Errorf("Config.applyLogLevelDefaults() LogLevel = %v, want %v", c.LogLevel, tt.want)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	// Create test credentials file first
	os.MkdirAll("testdata", 0755)
	testCredentials := `{"type": "service_account", "project_id": "test"}`
	os.WriteFile("testdata/test-credentials.json", []byte(testCredentials), 0644)
	defer os.RemoveAll("testdata")
	
	// Clear environment variables that might interfere
	originalEnv := make(map[string]string)
	envVars := []string{"TORN_API_KEY", "SPREADSHEET_ID", "ENV", "LOGLEVEL", "TORN_CREDENTIALS_FILE"}
	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	defer func() {
		for env, val := range originalEnv {
			if val != "" {
				os.Setenv(env, val)
			}
		}
	}()

	// Set required environment variables for the test
	os.Setenv("TORN_API_KEY", "test-key")
	os.Setenv("TORN_CREDENTIALS_FILE", "testdata/test-credentials.json")

	// Reset pflag state for clean test
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ExitOnError)
	os.Args = []string{"test"}

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Test defaults
	expectedDefaults := map[string]interface{}{
		"Output":          "stdout",
		"All":             false,
		"Both":            false,
		"RangeNOC":        "History!A1",
		"RangeAll":        "HistoryAll!A1",
		"Interval":        time.Duration(0),
		"CredentialsFile": "testdata/test-credentials.json",
		"Environment":     "development",
		"LogLevel":        "info", // Should be set by applyLogLevelDefaults
	}

	checkField := func(fieldName string, got, want interface{}) {
		if got != want {
			t.Errorf("Config.%s = %v, want %v", fieldName, got, want)
		}
	}

	checkField("Output", cfg.Output, expectedDefaults["Output"])
	checkField("All", cfg.All, expectedDefaults["All"])
	checkField("Both", cfg.Both, expectedDefaults["Both"])
	checkField("RangeNOC", cfg.RangeNOC, expectedDefaults["RangeNOC"])
	checkField("RangeAll", cfg.RangeAll, expectedDefaults["RangeAll"])
	checkField("Interval", cfg.Interval, expectedDefaults["Interval"])
	checkField("CredentialsFile", cfg.CredentialsFile, expectedDefaults["CredentialsFile"])
	checkField("Environment", cfg.Environment, expectedDefaults["Environment"])
	checkField("LogLevel", cfg.LogLevel, expectedDefaults["LogLevel"])
}