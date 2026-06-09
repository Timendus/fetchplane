package library

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeChars = regexp.MustCompile(`[^\w\s\-\.]`)

// safeFilename strips characters that are problematic in file names.
func safeFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	if len(s) > 180 {
		s = s[:180]
	}
	return s
}

func targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID string) (string, string) {
	dir := libraryDir(downloadDir, creatorName, channelName)
	name := displayTitle(postTitle, attachmentTitle)
	name = safeFilename(name)
	if name == "" {
		name = "video"
	}
	shortID := attachmentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return dir, filepath.Join(dir, fmt.Sprintf("%s [%s]", name, shortID))
}

func libraryDir(downloadDir, creatorName, channelName string) string {
	dirName := safeFilename(creatorName)
	if channel := safeFilename(channelName); channel != "" && channel != safeFilename(creatorName) {
		dirName = channel
	}
	return filepath.Join(downloadDir, dirName)
}

func displayTitle(postTitle, attachmentTitle string) string {
	name := postTitle
	if attachment := safeFilename(attachmentTitle); attachment != "" && attachment != safeFilename(postTitle) {
		name += " - " + attachmentTitle
	}
	return strings.TrimSpace(name)
}

func extensionForResponse(resp *http.Response, rawURL string) string {
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		if extensions, err := mime.ExtensionsByType(strings.Split(contentType, ";")[0]); err == nil && len(extensions) > 0 {
			if extensions[0] == ".jpe" || extensions[0] == ".jfif" {
				return ".jpg"
			}
			return extensions[0]
		}
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		if ext := filepath.Ext(parsed.Path); ext != "" && len(ext) <= 5 {
			if strings.EqualFold(ext, ".jpe") || strings.EqualFold(ext, ".jfif") {
				return ".jpg"
			}
			return ext
		}
	}
	return ".jpg"
}
