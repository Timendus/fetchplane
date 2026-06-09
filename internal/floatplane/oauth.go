package floatplane

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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
