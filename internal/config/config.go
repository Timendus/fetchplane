package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultPollInterval      = 600
	defaultMaxDaysToDownload = 7
	defaultStoragePath       = "/data"
)

type Config struct {
	StoragePath         string `yaml:"-"`
	ConfigFile          string `yaml:"-"`
	DownloadDir         string `yaml:"-"`
	SessionFile         string `yaml:"-"`
	OAuthClientID       string `yaml:"oauthClientId"`
	OAuthScope          string `yaml:"oauthScope"`
	PollInterval        int    `yaml:"pollInterval"`
	MaxDaysToDownload   int    `yaml:"maxDaysToDownload"`
	RemoveOlderThanDays int    `yaml:"removeOlderThanDays"`
}

func Load() (*Config, error) {
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = defaultStoragePath
	}

	cfg := &Config{
		StoragePath:       storagePath,
		ConfigFile:        filepath.Join(storagePath, "config.yaml"),
		DownloadDir:       filepath.Join(storagePath, "downloads"),
		SessionFile:       filepath.Join(storagePath, "session.json"),
		OAuthScope:        "openid videos:watch account:read",
		PollInterval:      defaultPollInterval,
		MaxDaysToDownload: defaultMaxDaysToDownload,
	}

	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage path: %w", err)
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("create downloads path: %w", err)
	}

	if err := ensureConfigFile(cfg); err != nil {
		return nil, err
	}
	if err := loadConfigFile(cfg); err != nil {
		return nil, err
	}
	if err := ensureSessionFile(cfg.SessionFile); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func ensureConfigFile(cfg *Config) error {
	if _, err := os.Stat(cfg.ConfigFile); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.WriteFile(cfg.ConfigFile, []byte(defaultConfigYAML(cfg)), 0644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	return nil
}

func loadConfigFile(cfg *Config) error {
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	return nil
}

func ensureSessionFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte("{\n  \"auth\": {}\n}\n"), 0644)
}

func defaultConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# Fetchplane configuration
#
# This file is created automatically on first run.
# Fill in oauthClientId before starting a real sync.

# Required: your Floatplane-issued OAuth client ID.
oauthClientId: ""

# Optional OAuth scope.
oauthScope: %q

# Polling interval in seconds when running with -daemon.
pollInterval: %d

# Only download missing videos from the last N days.
maxDaysToDownload: %d

# Remove managed videos older than N days.
# Set to 0 to keep videos indefinitely.
removeOlderThanDays: 0
`, cfg.OAuthScope, cfg.PollInterval, cfg.MaxDaysToDownload)
}

func validateConfig(cfg *Config) error {
	if cfg.OAuthClientID == "" {
		return fmt.Errorf("config.yaml is missing oauthClientId")
	}
	if cfg.OAuthScope == "" {
		cfg.OAuthScope = "openid videos:watch account:read"
	}
	if cfg.PollInterval <= 0 {
		return fmt.Errorf("config.yaml pollInterval must be a positive integer (seconds)")
	}
	if cfg.MaxDaysToDownload <= 0 {
		return fmt.Errorf("config.yaml maxDaysToDownload must be a positive integer")
	}
	if cfg.RemoveOlderThanDays < 0 {
		return fmt.Errorf("config.yaml removeOlderThanDays cannot be negative")
	}
	if cfg.RemoveOlderThanDays > 0 && cfg.MaxDaysToDownload > cfg.RemoveOlderThanDays {
		return fmt.Errorf("config.yaml maxDaysToDownload cannot be greater than removeOlderThanDays")
	}
	return nil
}
