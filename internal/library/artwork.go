package library

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func EnsureLibraryDir(downloadDir, creatorName, channelName string) (string, error) {
	dir := libraryDir(downloadDir, creatorName, channelName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

func SaveLibraryArtwork(fetch func(string) (*http.Response, error), downloadDir, creatorName, channelName, name, artworkURL string) (string, bool, error) {
	if artworkURL == "" || name == "" {
		return "", false, nil
	}

	dir, err := EnsureLibraryDir(downloadDir, creatorName, channelName)
	if err != nil {
		return "", false, err
	}

	prefix := filepath.Join(dir, name)
	if matches, err := filepath.Glob(prefix + ".*"); err == nil {
		for _, match := range matches {
			ext := strings.ToLower(filepath.Ext(match))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
				return match, false, nil
			}
		}
	}

	resp, err := fetch(artworkURL)
	if err != nil {
		return "", false, fmt.Errorf("GET artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("artwork server returned %d", resp.StatusCode)
	}

	filename := prefix + extensionForResponse(resp, artworkURL)
	f, err := os.Create(filename)
	if err != nil {
		return "", false, fmt.Errorf("create artwork file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(filename)
		return "", false, fmt.Errorf("write artwork: %w", err)
	}
	return filename, true, nil
}

func SaveThumbnail(fetch func(string) (*http.Response, error), downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, thumbnailURL string) (string, bool, error) {
	if thumbnailURL == "" {
		return "", false, nil
	}
	dir, basename := targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if matches, err := filepath.Glob(basename + ".*"); err == nil {
		for _, match := range matches {
			ext := strings.ToLower(filepath.Ext(match))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
				return match, false, nil
			}
		}
	}

	resp, err := fetch(thumbnailURL)
	if err != nil {
		return "", false, fmt.Errorf("GET thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("thumbnail server returned %d", resp.StatusCode)
	}

	ext := extensionForResponse(resp, thumbnailURL)
	filename := basename + ext
	f, err := os.Create(filename)
	if err != nil {
		return "", false, fmt.Errorf("create thumbnail file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(filename)
		return "", false, fmt.Errorf("write thumbnail: %w", err)
	}
	return filename, true, nil
}
