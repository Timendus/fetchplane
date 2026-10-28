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
	defaultLibraryPath       = "/downloads"
	defaultConfigPath        = "/config"
	defaultListenAddress     = ":38080"
)

type Config struct {
	LibraryPath         string `yaml:"-"`
	ConfigPath          string `yaml:"-"`
	ConfigFile          string `yaml:"-"`
	DownloadDir         string `yaml:"-"`
	SessionFile         string `yaml:"-"`
	Listen              string `yaml:"listen"`
	OAuthClientID       string `yaml:"oauthClientId"`
	OAuthScope          string `yaml:"oauthScope"`
	PollInterval        int    `yaml:"pollInterval"`
	MaxDaysToDownload   int    `yaml:"maxDaysToDownload"`
	RemoveOlderThanDays int    `yaml:"removeOlderThanDays"`
}

type Editable struct {
	PollInterval        int `yaml:"pollInterval" json:"pollInterval"`
	MaxDaysToDownload   int `yaml:"maxDaysToDownload" json:"maxDaysToDownload"`
	RemoveOlderThanDays int `yaml:"removeOlderThanDays" json:"removeOlderThanDays"`
}

func Load() (*Config, error) {
	libraryPath := os.Getenv("LIBRARY_PATH")
	if libraryPath == "" {
		libraryPath = defaultLibraryPath
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	cfg := &Config{
		LibraryPath:       libraryPath,
		ConfigPath:        configPath,
		ConfigFile:        filepath.Join(configPath, "config.yaml"),
		DownloadDir:       libraryPath,
		SessionFile:       filepath.Join(configPath, "session.json"),
		Listen:            defaultListenAddress,
		OAuthClientID:     "floatplane-downloader",
		OAuthScope:        "openid profile email offline_access",
		PollInterval:      defaultPollInterval,
		MaxDaysToDownload: defaultMaxDaysToDownload,
	}

	if err := os.MkdirAll(cfg.ConfigPath, 0755); err != nil {
		return nil, fmt.Errorf("create config path: %w", err)
	}
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("create library path: %w", err)
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

	if err := os.WriteFile(cfg.ConfigFile, []byte(renderConfigYAML(cfg)), 0644); err != nil {
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

func ReadEditable(path string) (*Editable, error) {
	editable := defaultEditable()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, editable); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return editable, nil
}

func WriteEditable(path string, editable *Editable) error {
	if err := ValidateEditable(editable); err != nil {
		return err
	}
	cfg := &Config{
		Listen:            defaultListenAddress,
		PollInterval:      defaultPollInterval,
		MaxDaysToDownload: defaultMaxDaysToDownload,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	cfg.PollInterval = editable.PollInterval
	cfg.MaxDaysToDownload = editable.MaxDaysToDownload
	cfg.RemoveOlderThanDays = editable.RemoveOlderThanDays
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(renderConfigYAML(cfg)), 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func defaultEditable() *Editable {
	return &Editable{
		PollInterval:      defaultPollInterval,
		MaxDaysToDownload: defaultMaxDaysToDownload,
	}
}

func renderConfigYAML(cfg *Config) string {
	return fmt.Sprintf(`# Fetchplane configuration
#
# This file is created automatically on first run.
# Fill in oauthClientId before starting a real sync.

# Required: your Floatplane-issued OAuth client ID.
oauthClientId: %q

# Optional OAuth scope.
oauthScope: %q

# HTTP listen address for the built-in web UI.
listen: %q

# Polling interval in seconds between subscription scans.
pollInterval: %d

# Only download missing videos from the last N days.
maxDaysToDownload: %d

# Remove managed videos older than N days.
# Set to 0 to keep videos indefinitely.
removeOlderThanDays: %d
`, cfg.OAuthClientID, cfg.OAuthScope, cfg.Listen, cfg.PollInterval, cfg.MaxDaysToDownload, cfg.RemoveOlderThanDays)
}

func validateConfig(cfg *Config) error {
	if cfg.OAuthClientID == "" {
		return fmt.Errorf("config.yaml is missing oauthClientId")
	}
	if cfg.Listen == "" {
		return fmt.Errorf("config.yaml is missing listen port")
	}
	if cfg.OAuthScope == "" {
		return fmt.Errorf("config.yaml is missing oauthScope")
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

func ValidateEditable(cfg *Editable) error {
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
