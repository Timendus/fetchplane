package downloader

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

func SaveNFO(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, showTitle, description string, released time.Time) (string, bool, error) {
	dir, basename := targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	filename := basename + ".nfo"
	if _, err := os.Stat(filename); err == nil {
		return filename, false, nil
	}

	description = cleanDescription(description)
	doc := episodeDetails{
		XMLName:     xml.Name{Local: "episodedetails"},
		Title:       displayTitle(postTitle, attachmentTitle),
		ShowTitle:   showTitle,
		Description: description,
		Plot:        description,
		Aired:       released.Format("2006-01-02"),
		Season:      released.Format("2006"),
		Episode:     released.Format("0102150405"),
	}
	data, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", false, fmt.Errorf("marshal nfo: %w", err)
	}
	data = append([]byte(xml.Header), data...)
	data = append(data, '\n')
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", false, fmt.Errorf("write nfo: %w", err)
	}
	return filename, true, nil
}

func SaveSubtitles(fetch func(string) (*http.Response, error), downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID string, tracks []SubtitleTrack) error {
	dir, basename := targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	for _, track := range tracks {
		if track.URL == "" {
			continue
		}

		suffix := ""
		if lang := safeFilename(track.Language); lang != "" {
			suffix = "." + strings.ToLower(lang)
		}
		filename := basename + suffix + ".vtt"
		if _, err := os.Stat(filename); err == nil {
			continue
		}

		resp, err := fetch(track.URL)
		if err != nil {
			return fmt.Errorf("GET subtitle: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("subtitle server returned %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read subtitle: %w", err)
		}
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("write subtitle: %w", err)
		}
	}

	return nil
}

type episodeDetails struct {
	XMLName     xml.Name `xml:"episodedetails"`
	Title       string   `xml:"title"`
	ShowTitle   string   `xml:"showtitle"`
	Description string   `xml:"description"`
	Plot        string   `xml:"plot"`
	Aired       string   `xml:"aired"`
	Season      string   `xml:"season"`
	Episode     string   `xml:"episode"`
}

type SubtitleTrack struct {
	URL      string
	Language string
}

func targetPathParts(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID string) (string, string) {
	dirName := safeFilename(creatorName)
	if channel := safeFilename(channelName); channel != "" && channel != safeFilename(creatorName) {
		dirName = channel
	}
	dir := filepath.Join(downloadDir, dirName)
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

func cleanDescription(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n"))
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
