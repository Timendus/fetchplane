package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	OAuthClientID       string
	OAuthScope          string
	DownloadDir         string
	SessionFile         string
	PollInterval        int
	MaxDaysToDownload   int
	RemoveOlderThanDays int
}

func Load() (*Config, error) {
	c := &Config{
		OAuthClientID:     os.Getenv("FP_OAUTH_CLIENT_ID"),
		OAuthScope:        os.Getenv("FP_OAUTH_SCOPE"),
		DownloadDir:       os.Getenv("FP_DOWNLOAD_DIR"),
		SessionFile:       os.Getenv("FP_SESSION_FILE"),
		PollInterval:      300,
		MaxDaysToDownload: 7,
	}

	if c.OAuthClientID == "" {
		return nil, fmt.Errorf("FP_OAUTH_CLIENT_ID is required")
	}
	if c.OAuthScope == "" {
		c.OAuthScope = "openid videos:watch account:read"
	}
	if c.DownloadDir == "" {
		c.DownloadDir = "downloads"
	}
	if c.SessionFile == "" {
		c.SessionFile = filepath.Join(c.DownloadDir, ".fetchplane-session.json")
	}

	if s := os.Getenv("FP_POLL_INTERVAL"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("FP_POLL_INTERVAL must be a positive integer (seconds)")
		}
		c.PollInterval = v
	}

	if s := os.Getenv("FP_MAX_DAYS_TO_DOWNLOAD"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("FP_MAX_DAYS_TO_DOWNLOAD must be a positive integer")
		}
		c.MaxDaysToDownload = v
	}

	if s := os.Getenv("FP_REMOVE_OLDER_THAN_DAYS"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("FP_REMOVE_OLDER_THAN_DAYS must be a positive integer")
		}
		c.RemoveOlderThanDays = v
	}

	if c.MaxDaysToDownload > 0 && c.RemoveOlderThanDays > 0 && c.MaxDaysToDownload > c.RemoveOlderThanDays {
		return nil, fmt.Errorf("FP_MAX_DAYS_TO_DOWNLOAD cannot be greater than FP_REMOVE_OLDER_THAN_DAYS")
	}

	return c, nil
}
