package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"time"
)

func (c *Client) waitForDeliverySlot() {
	c.deliveryMu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(c.nextDelivery) {
		wait = time.Until(c.nextDelivery)
		now = c.nextDelivery
	}
	c.nextDelivery = now.Add(deliveryInterval)
	c.deliveryMu.Unlock()

	if wait > 0 {
		log.Printf("delivery pacing: sleeping %s before next delivery/info request", wait.Round(time.Millisecond))
		time.Sleep(wait)
	}
}

// GetDeliveryURL returns the preferred download URL for a video entity ID.
func (c *Client) GetDeliveryURL(entityID string) (string, error) {
	c.waitForDeliverySlot()

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

	if resp.StatusCode != 200 {
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
			if !v.Enabled || v.Hidden || v.URL == "" {
				continue
			}

			variantURL := v.URL
			if v.URL[0] == '/' {
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
