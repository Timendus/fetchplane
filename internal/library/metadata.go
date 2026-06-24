package library

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

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

func SaveVideoSidecars(fetchMedia func(string) (*http.Response, error), fetchPublic func(string) (*http.Response, error), downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, showTitle, description string, released time.Time, thumbnailURL string, tracks []SubtitleTrack) (string, error) {
	thumbnailPath, _, err := SaveThumbnail(fetchMedia, downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, thumbnailURL)
	if err != nil {
		return thumbnailPath, err
	}

	if _, _, err := SaveNFO(downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, showTitle, description, released); err != nil {
		return thumbnailPath, err
	}

	if err := SaveSubtitles(fetchPublic, downloadDir, creatorName, channelName, postTitle, attachmentTitle, attachmentID, tracks); err != nil {
		return thumbnailPath, err
	}

	return thumbnailPath, nil
}

func cleanDescription(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(input, "\r\n", "\n"), "\r", "\n"))
}
