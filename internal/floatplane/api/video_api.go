package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

func (c *Client) GetVideoInfo(videoID string) (*VideoInfo, error) {
	u, _ := url.Parse(baseURL + "/api/v3/content/video")
	q := u.Query()
	q.Set("id", videoID)
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("video info failed (%d): %s", resp.StatusCode, string(raw))
	}

	var video VideoInfo
	if err := json.NewDecoder(resp.Body).Decode(&video); err != nil {
		return nil, fmt.Errorf("video info decode error: %w", err)
	}
	return &video, nil
}

func (c *Client) GetBlogPosts(creatorID string, limit, fetchAfter int) ([]BlogPost, error) {
	u, _ := url.Parse(baseURL + "/api/v3/content/creator")
	q := u.Query()
	q.Set("id", creatorID)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("fetchAfter", strconv.Itoa(fetchAfter))
	q.Set("hasVideo", "true")
	q.Set("sort", "DESC")
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("content list failed (%d): %s", resp.StatusCode, string(raw))
	}

	var result []BlogPost
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("content list decode error: %w", err)
	}
	return result, nil
}

// GetBlogPost returns the complete post, including its video attachments.
//
// The creator content listing is intentionally lightweight and no longer
// includes attachment data, even when hasVideo=true. Fetch a post separately
// before deciding which downloads to create.
func (c *Client) GetBlogPost(postID string) (*BlogPost, error) {
	u, _ := url.Parse(baseURL + "/api/v3/content/post")
	q := u.Query()
	q.Set("id", postID)
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("content post failed (%d): %s", resp.StatusCode, string(raw))
	}

	var post BlogPost
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return nil, fmt.Errorf("content post decode error: %w", err)
	}
	return &post, nil
}
