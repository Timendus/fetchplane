package api

import (
	"encoding/json"
	"sort"
)

type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Subscriptions

type Subscription struct {
	Creator string `json:"creator"`
}

type CreatorInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URLName string `json:"urlname"`
}

type BlogPost struct {
	ID               string               `json:"id"`
	GUID             string               `json:"guid"`
	Title            string               `json:"title"`
	Text             string               `json:"text"`
	ReleaseDate      string               `json:"releaseDate"` // e.g. "2024-01-15T12:00:00.000Z"
	Thumbnail        *Thumbnail           `json:"thumbnail"`
	Channel          ChannelRef           `json:"channel"`
	AttachmentOrder  []string             `json:"attachmentOrder"`
	VideoAttachments []VideoAttachmentRef `json:"videoAttachments"`
}

type Thumbnail struct {
	Path string `json:"path"`
}

type ChannelRef struct {
	ID    string
	Title string
}

func (c *ChannelRef) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		c.ID = id
		return nil
	}

	var raw struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.ID = raw.ID
	c.Title = raw.Title
	return nil
}

type VideoAttachment struct {
	ID    string
	Title string
}

type VideoInfo struct {
	ID         string      `json:"id"`
	GUID       string      `json:"guid"`
	Title      string      `json:"title"`
	TextTracks []TextTrack `json:"textTracks"`
}

type TextTrack struct {
	Src      string `json:"src"`
	Language string `json:"language"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
}

type VideoAttachmentRef struct {
	ID    string
	Title string
}

func (v *VideoAttachmentRef) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		v.ID = id
		return nil
	}

	var raw struct {
		ID    string `json:"id"`
		GUID  string `json:"guid"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	v.ID = firstNonEmptyString(raw.ID, raw.GUID)
	v.Title = raw.Title
	return nil
}

func (p BlogPost) Attachments() []VideoAttachment {
	result := make([]VideoAttachment, 0, len(p.VideoAttachments))
	order := make(map[string]int, len(p.AttachmentOrder))
	for idx, id := range p.AttachmentOrder {
		if id != "" {
			order[id] = idx
		}
	}

	for _, attachment := range p.VideoAttachments {
		if attachment.ID == "" {
			continue
		}
		result = append(result, VideoAttachment{
			ID:    attachment.ID,
			Title: attachment.Title,
		})
	}

	if len(order) > 0 {
		sort.SliceStable(result, func(i, j int) bool {
			ii, iok := order[result[i].ID]
			jj, jok := order[result[j].ID]
			switch {
			case iok && jok:
				return ii < jj
			case iok:
				return true
			case jok:
				return false
			default:
				return result[i].ID < result[j].ID
			}
		})
	}

	return result
}

// Delivery (v3)

type DeliveryResponse struct {
	Groups []DeliveryGroup `json:"groups"`
}

type DeliveryGroup struct {
	Origins  []DeliveryOrigin  `json:"origins"`
	Variants []DeliveryVariant `json:"variants"`
}

type DeliveryOrigin struct {
	URL string `json:"url"`
}

type DeliveryVariant struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	Order    int    `json:"order"`
	Enabled  bool   `json:"enabled"`
	Hidden   bool   `json:"hidden"`
	MimeType string `json:"mimeType"`
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
