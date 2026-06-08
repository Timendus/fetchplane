package downloader

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

// Download streams a video from videoURL to a stable per-attachment filename.
// Returns the target filename, whether it was downloaded, and any error.
func Download(fetch func(string) (*http.Response, error), downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, videoURL string) (string, bool, error) {
	dir := filepath.Join(downloadDir, safeFilename(creatorName))
	if channel := safeFilename(channelName); channel != "" && channel != safeFilename(creatorName) {
		dir = filepath.Join(dir, channel)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	name := safeFilename(postTitle)
	if attachment := safeFilename(attachmentTitle); attachment != "" && attachment != name {
		name += " - " + attachment
	}
	if name == "" {
		name = "video"
	}
	shortID := attachmentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s [%s].mp4", name, shortID))

	if _, err := os.Stat(filename); err == nil {
		log.Printf("skip (exists): %s", filename)
		return filename, false, nil
	}

	log.Printf("downloading: %s", filename)

	resp, err := fetch(videoURL)
	if err != nil {
		return "", false, fmt.Errorf("GET video: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("video server returned %d", resp.StatusCode)
	}

	// Write to a temp file first so a partial download is never left as a
	// completed file.
	tmp := filename + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", false, fmt.Errorf("create temp file: %w", err)
	}

	written, err := copyWithProgress(f, resp.Body, resp.ContentLength, filename)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", false, fmt.Errorf("write video: %w", err)
	}

	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return "", false, fmt.Errorf("rename temp file: %w", err)
	}

	log.Printf("done: %s (%.1f MB)", filename, float64(written)/1024/1024)
	return filename, true, nil
}

// copyWithProgress copies src to dst, logging progress every ~10%.
func copyWithProgress(dst io.Writer, src io.Reader, total int64, label string) (int64, error) {
	buf := make([]byte, 1024*1024) // 1 MB buffer
	var written int64
	lastPct := -1

	for {
		n, err := src.Read(buf)
		if n > 0 {
			nw, werr := dst.Write(buf[:n])
			written += int64(nw)
			if werr != nil {
				return written, werr
			}

			if total > 0 {
				pct := int(float64(written) / float64(total) * 100)
				pct = (pct / 10) * 10 // round to nearest 10%
				if pct != lastPct {
					log.Printf("  %s — %d%% (%.1f / %.1f MB)",
						label, pct,
						float64(written)/1024/1024,
						float64(total)/1024/1024,
					)
					lastPct = pct
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
