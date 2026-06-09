package library

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Download streams a video from videoURL to a stable per-attachment filename.
// Returns the target filename, whether it was downloaded, and any error.
func Download(fetch func(string) (*http.Response, error), downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, videoURL string) (string, bool, error) {
	dir, _ := targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	filename := VideoPath(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)

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

func VideoPath(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID string) string {
	_, basename := targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	return basename + ".mp4"
}

func VideoExists(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID string) (string, bool) {
	filename := VideoPath(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	if _, err := os.Stat(filename); err == nil {
		return filename, true
	}
	return filename, false
}

// copyWithProgress copies src to dst, logging progress every ~10%.
func copyWithProgress(dst io.Writer, src io.Reader, total int64, label string) (int64, error) {
	buf := make([]byte, 1024*1024)
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
				pct = (pct / 10) * 10
				if pct != lastPct {
					log.Printf("  %s - %d%% (%.1f / %.1f MB)",
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
