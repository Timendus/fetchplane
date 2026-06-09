package floatplane

import (
	"net/http"
	"sync"
	"time"
)

const (
	baseURL          = "https://www.floatplane.com"
	authURL          = "https://auth.floatplane.com/realms/floatplane/protocol/openid-connect"
	userAgent        = "FetchPlane v0.1, CFNetwork"
	deliveryInterval = 31 * time.Second
)

type Client struct {
	http         *http.Client
	accessToken  string
	debug        bool
	deliveryMu   sync.Mutex
	nextDelivery time.Time
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) SetAccessToken(token string) {
	c.accessToken = token
}

func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}
