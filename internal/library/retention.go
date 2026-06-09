package library

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type nfoEpisodeDetails struct {
	Aired string `xml:"aired"`
}

func RemoveOlderThan(dir string, cutoff time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".mp4" {
			continue
		}

		videoPath := filepath.Join(dir, entry.Name())
		released, ok := releaseDateForVideo(videoPath)
		if !ok || released.After(cutoff) {
			continue
		}

		if err := removeVideoSet(videoPath); err != nil {
			return removed, err
		}
		removed++
	}

	return removed, nil
}

func releaseDateForVideo(videoPath string) (time.Time, bool) {
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
	if data, err := os.ReadFile(nfoPath); err == nil {
		var details nfoEpisodeDetails
		if err := xml.Unmarshal(data, &details); err == nil && details.Aired != "" {
			if aired, err := time.Parse("2006-01-02", details.Aired); err == nil {
				return aired, true
			}
		}
	}

	info, err := os.Stat(videoPath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func removeVideoSet(videoPath string) error {
	pattern := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, path := range matches {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}
