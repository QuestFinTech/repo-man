// config/config.go - Configuration loading and management.
//
// This file handles loading configuration from a JSON file and environment variables.
// It defines the Config struct and functions to load and validate configuration.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

const ServerVersion = "0.1.0" // Define software version

// Config holds the application configuration.
type Config struct {
	LogFilePath      string `json:"log_file_path"`
	APIServerAddress string `json:"api_listener"`
	DataPath         string `json:"data_path"`
	RepositoryPath   string `json:"repository_path"`
	ShutdownDelay    int    `json:"shutdown_delay_seconds"`
	ConfigFileUsed   string `json:"-"` // Not from config file, but tracked for info
}

// Default configuration values if not provided in file or env vars.
const (
	defaultLogFilePath      = "gemini.rel-man.log"
	defaultAPIServerAddress = ":8080"
	defaultDataPath         = "./data"
	defaultRepositoryPath   = "./repository"
	defaultShutdownDelay    = 5
	configFileName          = "gemini.rel-man.config.json"
)

// LoadConfig loads the configuration from a JSON file and environment variables.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()
	configFilePath := getConfigFilePath()

	if err := loadConfigFile(cfg, configFilePath); err != nil {
		if !os.IsNotExist(err) { // Ignore file not found error, use defaults or env vars
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		fmt.Println("Configuration file not found, using default values and environment variables.")
	} else {
		cfg.ConfigFileUsed = configFilePath // Track config file used if loaded successfully
		fmt.Printf("Configuration loaded from file: %s\n", configFilePath)
	}

	applyEnvironmentVariables(cfg) // Override with environment variables if set

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns a Config struct with default values.
func DefaultConfig() *Config {
	return &Config{
		LogFilePath:      defaultLogFilePath,
		APIServerAddress: defaultAPIServerAddress,
		DataPath:         defaultDataPath,
		RepositoryPath:   defaultRepositoryPath,
		ShutdownDelay:    defaultShutdownDelay,
	}
}

// getConfigFilePath determines the configuration file path.
// It checks for environment variable QFT_RELMAN_CONFIG_PATH first, then defaults to configFileName.
func getConfigFilePath() string {
	if envPath := os.Getenv("QFT_RELMAN_CONFIG_PATH"); envPath != "" {
		return envPath
	}
	return configFileName
}

// loadConfigFile loads configuration from the JSON file.
func loadConfigFile(cfg *Config, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	return nil
}

// applyEnvironmentVariables overrides configuration with environment variables.
func applyEnvironmentVariables(cfg *Config) {
	setIfEnvExists(&cfg.LogFilePath, "QFT_RELMAN_LOG_FILE_PATH")
	setIfEnvExists(&cfg.APIServerAddress, "QFT_RELMAN_API_ADDRESS")
	setIfEnvExists(&cfg.DataPath, "QFT_RELMAN_DATA_PATH")
	setIfEnvExists(&cfg.RepositoryPath, "QFT_RELMAN_REPO_PATH")
	if val := os.Getenv("QFT_RELMAN_SHUTDOWN_DELAY"); val != "" {
		if delay, err := strconv.Atoi(val); err == nil {
			cfg.ShutdownDelay = delay
		} else {
			fmt.Printf("Warning: Invalid value for QFT_RELMAN_SHUTDOWN_DELAY, using default. Error: %v\n", err)
		}
	}
}

// setIfEnvExists sets the config value from environment variable if it exists.
func setIfEnvExists(configValue *string, envName string) {
	if val := os.Getenv(envName); val != "" {
		*configValue = val
	}
}

// validateConfig performs basic validation of the configuration.
func validateConfig(cfg *Config) error {
	if cfg.APIServerAddress == "" {
		return fmt.Errorf("API listener address cannot be empty")
	}
	if cfg.DataPath == "" {
		return fmt.Errorf("data path cannot be empty")
	}
	if cfg.RepositoryPath == "" {
		return fmt.Errorf("repository path cannot be empty")
	}
	if cfg.ShutdownDelay < 0 {
		return fmt.Errorf("shutdown delay must be non-negative")
	}
	return nil
}

// SetupLogger initializes the logger and log file.
func SetupLogger(logFilePath string) (*log.Logger, *os.File, error) {
	logDir := filepath.Dir(logFilePath)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(logFile, "QFT RelMan: ", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("Logger initialized.") // Initial log message

	return logger, logFile, nil
}
