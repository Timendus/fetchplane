package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

func (c *Client) GetSubscriptions() ([]Subscription, error) {
	resp, err := c.get(baseURL + "/api/v3/user/subscriptions")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

	if resp.StatusCode != 200 {
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

func (c *Client) GetNamedCreator(urlName string) (*CreatorInfo, error) {
	if urlName == "" {
		return nil, fmt.Errorf("creator urlname is empty")
	}

	u, _ := url.Parse(baseURL + "/api/v3/creator/named")
	q := u.Query()
	q.Set("creatorURL", urlName)
	u.RawQuery = q.Encode()

	resp, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("creator named failed (%d): %s", resp.StatusCode, string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("creator named read error: %w", err)
	}

	var creators []CreatorInfo
	if err := json.Unmarshal(raw, &creators); err == nil {
		for _, creator := range creators {
			if creator.ID == "" {
				creator.ID = urlName
			}
			if creator.URLName == "" {
				creator.URLName = urlName
			}
			return &creator, nil
		}
		return nil, fmt.Errorf("creator named returned no creators")
	}

	var creator CreatorInfo
	if err := json.Unmarshal(raw, &creator); err != nil {
		return nil, fmt.Errorf("creator named decode error: %w", err)
	}
	if creator.ID == "" {
		creator.ID = urlName
	}
	if creator.URLName == "" {
		creator.URLName = urlName
	}
	return &creator, nil
}
