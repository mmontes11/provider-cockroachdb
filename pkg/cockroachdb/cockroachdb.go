package cockroachdb

import (
	"fmt"
	"net/http"
	"net/url"
)

const (
	defaultURL = "https://cockroachlabs.cloud/api"
)

// ClientOption provides a variadic option for configuring the client
type ClientOption func(c *Client) error

// WithBaseURL allows to override default base URL
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) error {
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("error parsing URL: %v", err)
		}

		c.baseURL = parsedURL
		return nil
	}
}

// WithHTTPClient allows to override default HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) error {
		if client == nil {
			client = http.DefaultClient
		}

		c.client = client
		return nil
	}
}

// Client performs requests on the Cocokroachdb Cloud API
type Client struct {
	client  *http.Client
	baseURL *url.URL
}

// NewClient creates a new instance of the client
func NewClient(opts ...ClientOption) (*Client, error) {
	baseURL, err := url.Parse(defaultURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	client := Client{
		client:  http.DefaultClient,
		baseURL: baseURL,
	}
	for _, opt := range opts {
		if err := opt(&client); err != nil {
			return nil, fmt.Errorf("error setting option: %v", err)
		}
	}

	return &client, nil
}
