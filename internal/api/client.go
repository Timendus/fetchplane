package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL   = "https://www.floatplane.com"
	authURL   = "https://auth.floatplane.com/realms/floatplane/protocol/openid-connect"
	userAgent = "FetchPlane v0.1, CFNetwork"
)

type Client struct {
	http        *http.Client
	accessToken string
}

func NewClient() (*Client, error) {
	return &Client{
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (c *Client) SetAccessToken(token string) {
	c.accessToken = token
}

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

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		retryAfter := 5 * time.Second
		if value := resp.Header.Get("Retry-After"); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				retryAfter = time.Duration(seconds) * time.Second
			}
		}
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

func (c *Client) decodeOAuthError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)
	var oauthErr OAuthError
	if err := json.Unmarshal(raw, &oauthErr); err == nil && oauthErr.Error != "" {
		if oauthErr.ErrorDescription != "" {
			return fmt.Errorf("%s: %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return fmt.Errorf("%s", oauthErr.Error)
	}
	return fmt.Errorf("%s", strings.TrimSpace(string(raw)))
}

func (c *Client) BeginDeviceAuthorization(clientID, scope string) (*DeviceAuthorization, error) {
	resp, err := c.postForm(authURL+"/auth/device", url.Values{
		"client_id": {clientID},
		"scope":     {scope},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed (%d): %w", resp.StatusCode, c.decodeOAuthError(resp))
	}

	var result DeviceAuthorization
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("device authorization decode error: %w", err)
	}
	if result.DeviceCode == "" || result.VerificationURI == "" {
		return nil, fmt.Errorf("device authorization response missing required fields")
	}
	return &result, nil
}

func (c *Client) PollDeviceAuthorization(clientID, deviceCode string) (*OAuthToken, *OAuthError, error) {
	resp, err := c.postForm(authURL+"/token", url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {clientID},
		"device_code": {deviceCode},
	})
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var token OAuthToken
		if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
			return nil, nil, fmt.Errorf("token decode error: %w", err)
		}
		return &token, nil, nil
	}

	var oauthErr OAuthError
	if err := json.NewDecoder(resp.Body).Decode(&oauthErr); err != nil {
		return nil, nil, fmt.Errorf("token error decode failed (%d): %w", resp.StatusCode, err)
	}
	if oauthErr.Error == "" {
		oauthErr.Error = fmt.Sprintf("http_%d", resp.StatusCode)
	}
	return nil, &oauthErr, nil
}

func (c *Client) RefreshToken(clientID, refreshToken string) (*OAuthToken, error) {
	resp, err := c.postForm(authURL+"/token", url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (%d): %w", resp.StatusCode, c.decodeOAuthError(resp))
	}

	var token OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("token refresh decode error: %w", err)
	}
	return &token, nil
}

func (c *Client) GetSubscriptions() ([]Subscription, error) {
	resp, err := c.get(baseURL + "/api/v3/user/subscriptions")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subscriptions failed (%d): %s", resp.StatusCode, string(raw))
	}

	var subs []Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return nil, fmt.Errorf("subscriptions decode error: %w", err)
	}
	return subs, nil
}

func (c *Client) GetCreatorInfo(creatorID string) (*CreatorInfo, error) {
	u, _ := url.Parse(baseURL + "/api/v3/creator/info")
	q := u.Query()
	q.Set("id", creatorID)
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("creator info failed (%d): %s", resp.StatusCode, string(raw))
	}

	var creator CreatorInfo
	if err := json.NewDecoder(resp.Body).Decode(&creator); err != nil {
		return nil, fmt.Errorf("creator info decode error: %w", err)
	}
	if creator.ID == "" {
		creator.ID = creatorID
	}
	return &creator, nil
}

func (c *Client) GetBlogPosts(creatorID string, limit, fetchAfter int) ([]BlogPost, error) {
	u, _ := url.Parse(baseURL + "/api/v3/content/creator")
	q := u.Query()
	q.Set("id", creatorID)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("fetchAfter", fmt.Sprintf("%d", fetchAfter))
	q.Set("hasVideo", "true")
	q.Set("sort", "DESC")
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("content list failed (%d): %s", resp.StatusCode, string(raw))
	}

	var result []BlogPost
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("content list decode error: %w", err)
	}
	return result, nil
}

// GetDeliveryURL returns the highest-quality MP4 download URL for a video entity ID.
func (c *Client) GetDeliveryURL(entityID string) (string, error) {
	u, _ := url.Parse(baseURL + "/api/v3/delivery/info")
	q := u.Query()
	q.Set("entityId", entityID)
	q.Set("scenario", "download")
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("delivery failed (%d): %s", resp.StatusCode, string(raw))
	}

	var delivery DeliveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&delivery); err != nil {
		return "", fmt.Errorf("delivery decode error: %w", err)
	}

	return pickBestURL(delivery)
}

// pickBestURL selects the highest-order enabled, non-hidden variant, preferring MP4.
func pickBestURL(d DeliveryResponse) (string, error) {
	bestMP4Order := -1
	bestMP4URL := ""
	bestAnyOrder := -1
	bestAnyURL := ""

	for _, group := range d.Groups {
		originBase := ""
		if len(group.Origins) > 0 {
			originBase = group.Origins[0].URL
		}

		for _, v := range group.Variants {
			if !v.Enabled || v.Hidden {
				continue
			}
			if v.URL == "" {
				continue
			}

			variantURL := v.URL
			if len(v.URL) > 0 && v.URL[0] == '/' {
				variantURL = originBase + v.URL
			}

			if v.Order > bestAnyOrder {
				bestAnyOrder = v.Order
				bestAnyURL = variantURL
			}
			if v.MimeType == "video/mp4" && v.Order > bestMP4Order {
				bestMP4Order = v.Order
				bestMP4URL = variantURL
			}
		}
	}

	if bestMP4URL != "" {
		return bestMP4URL, nil
	}
	if bestAnyURL != "" {
		return bestAnyURL, nil
	}
	return "", fmt.Errorf("no downloadable variants found in delivery response")
}
