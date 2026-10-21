package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetBlogPostFetchesFullVideoAttachments(t *testing.T) {
	t.Parallel()

	client := NewClient()
	client.http = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got, want := req.URL.Path, "/api/v3/content/post"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		if got, want := req.URL.Query().Get("id"), "post-1"; got != want {
			t.Errorf("post ID = %q, want %q", got, want)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"id":"post-1",
				"title":"A post",
				"videoAttachments":[{"id":"video-1","title":"The video"}],
				"attachmentOrder":["video-1"]
			}`)),
			Header: make(http.Header),
		}, nil
	})}

	post, err := client.GetBlogPost("post-1")
	if err != nil {
		t.Fatalf("GetBlogPost returned error: %v", err)
	}
	attachments := post.Attachments()
	if len(attachments) != 1 {
		t.Fatalf("attachment count = %d, want 1", len(attachments))
	}
	if got, want := attachments[0].ID, "video-1"; got != want {
		t.Errorf("attachment ID = %q, want %q", got, want)
	}
	if got, want := attachments[0].Title, "The video"; got != want {
		t.Errorf("attachment title = %q, want %q", got, want)
	}
}
