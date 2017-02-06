package mapbox

import (
	"net/http"
)

type Option interface {
	apply(*Client)
}

type withHTTPClient struct {
	hc *http.Client
}

func (whc *withHTTPClient) apply(c *Client) {
	c.httpClient = whc.hc
}

func WithHTTPClient(c *http.Client) Option {
	return &withHTTPClient{c}
}
