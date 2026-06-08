package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	OAuthClientID string
	OAuthScope    string
	DownloadDir   string
	StateFile     string
	PollInterval  int       // seconds
	Since         time.Time // zero value means no cutoff
}

func Load() (*Config, error) {
	c := &Config{
		OAuthClientID: os.Getenv("FP_OAUTH_CLIENT_ID"),
		OAuthScope:    os.Getenv("FP_OAUTH_SCOPE"),
		DownloadDir:   os.Getenv("FP_DOWNLOAD_DIR"),
		StateFile:     os.Getenv("FP_STATE_FILE"),
		PollInterval:  300,
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
	if c.StateFile == "" {
		c.StateFile = filepath.Join(c.DownloadDir, ".fetchplane-state.json")
	}

	if s := os.Getenv("FP_POLL_INTERVAL"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("FP_POLL_INTERVAL must be a positive integer (seconds)")
		}
		c.PollInterval = v
	}

	if s := os.Getenv("FP_SINCE"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			// Also accept plain date
			t, err = time.Parse("2006-01-02", s)
			if err != nil {
				return nil, fmt.Errorf("FP_SINCE must be RFC3339 (2006-01-02T15:04:05Z) or a date (2006-01-02)")
			}
		}
		c.Since = t
	}

	return c, nil
}
