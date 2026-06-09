package floatplane

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json,text/html")
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		start := time.Now()
		if c.debug {
			log.Printf("http request: %s %s", req.Method, req.URL.String())
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if c.debug {
				log.Printf("http error: %s %s (%s): %v", req.Method, req.URL.String(), time.Since(start).Round(time.Millisecond), err)
			}
			return nil, err
		}
		if c.debug {
			log.Printf(
				"http response: %s %s -> %d (%s, %d bytes)",
				req.Method,
				req.URL.String(),
				resp.StatusCode,
				time.Since(start).Round(time.Millisecond),
				resp.ContentLength,
			)
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		retryAfter := 5 * time.Second
		if value := resp.Header.Get("Retry-After"); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				retryAfter = time.Duration(seconds) * time.Second
			} else if retryTime, err := http.ParseTime(value); err == nil {
				if delay := time.Until(retryTime); delay > 0 {
					retryAfter = delay
				}
			}
		}
		log.Printf("http rate limit: %s %s -> sleeping %s before retry", req.Method, req.URL.String(), retryAfter.Round(time.Millisecond))
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		time.Sleep(retryAfter)
	}

	return nil, fmt.Errorf("request to %s was rate-limited too many times", req.URL)
}

func (c *Client) get(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) GetMedia(rawURL string) (*http.Response, error) {
	if strings.HasPrefix(rawURL, "/") {
		rawURL = baseURL + rawURL
	}
	return c.get(rawURL)
}

func (c *Client) GetPublic(rawURL string) (*http.Response, error) {
	if strings.HasPrefix(rawURL, "/") {
		rawURL = baseURL + rawURL
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	start := time.Now()
	if c.debug {
		log.Printf("http request: %s %s", req.Method, req.URL.String())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if c.debug {
			log.Printf("http error: %s %s (%s): %v", req.Method, req.URL.String(), time.Since(start).Round(time.Millisecond), err)
		}
		return nil, err
	}
	if c.debug {
		log.Printf(
			"http response: %s %s -> %d (%s, %d bytes)",
			req.Method,
			req.URL.String(),
			resp.StatusCode,
			time.Since(start).Round(time.Millisecond),
			resp.ContentLength,
		)
	}
	return resp, nil
}

func (c *Client) postForm(rawURL string, values url.Values) (*http.Response, error) {
	body := strings.NewReader(values.Encode())
	req, err := http.NewRequest(http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}
