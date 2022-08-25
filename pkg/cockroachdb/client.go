package cockroachdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	defaultURL    = "https://cockroachlabs.cloud/api"
	jsonMediaType = "application/json"
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

// WithAccessToken allows to override default HTTP client
func WithAccessToken(accessToken string) ClientOption {
	return func(c *Client) error {
		if accessToken == "" {
			return fmt.Errorf("access token must not be empty")
		}
		if c.client.Transport == nil {
			c.client.Transport = http.DefaultTransport
		}

		c.client.Transport = &accessTokenTransport{
			rt:          c.client.Transport,
			accessToken: accessToken,
		}
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

func (c *Client) do(ctx context.Context, req *http.Request, val interface{}) (*http.Response, error) {
	reqWithCtx := req.WithContext(ctx)
	res, err := c.client.Do(reqWithCtx)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	return res, c.handleResponse(ctx, res, val)
}

func (c *Client) get(ctx context.Context, path string, val interface{}) (*http.Response, error) {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating GET request: %v", err)
	}
	return c.do(ctx, req, val)
}

func (c *Client) post(ctx context.Context, path string, body, val interface{}) (*http.Response, error) {
	req, err := c.newRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, fmt.Errorf("error creating POST request: %v", err)
	}
	return c.do(ctx, req, val)
}

func (c *Client) patch(ctx context.Context, path string, body, val interface{}) (*http.Response, error) {
	req, err := c.newRequest(http.MethodPatch, path, body)
	if err != nil {
		return nil, fmt.Errorf("error creating PATCH request: %v", err)
	}
	return c.do(ctx, req, val)
}

func (c *Client) delete(ctx context.Context, path string, body, val interface{}) (*http.Response, error) {
	req, err := c.newRequest(http.MethodPatch, path, body)
	if err != nil {
		return nil, fmt.Errorf("error creating DELETE request: %v", err)
	}
	return c.do(ctx, req, val)
}

func (c *Client) handleResponse(ctx context.Context, res *http.Response, val interface{}) error {
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	if res.StatusCode >= 400 {
		var errResponse errorResponse
		if err := json.Unmarshal(bytes, &errResponse); err != nil {
			return fmt.Errorf("error umarshalling response error: %v", err)
		}
		return &Error{
			ErrorCode: errResponse.Code,
			HTTPCode:  res.StatusCode,
			Message:   errResponse.Message,
		}
	}

	if val == nil {
		return nil
	}
	if err := json.Unmarshal(bytes, &val); err != nil {
		return fmt.Errorf("error umarshalling response error: %v", err)
	}
	return nil
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	url, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	if method == http.MethodGet {
		req, err := http.NewRequest(method, url.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}
		return req, nil
	}

	buffer := new(bytes.Buffer)
	if body != nil {
		if err := json.NewEncoder(buffer).Encode(body); err != nil {
			return nil, fmt.Errorf("error encoding body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url.String(), buffer)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", jsonMediaType)
	req.Header.Set("Accept", jsonMediaType)

	return req, nil
}

type accessTokenTransport struct {
	rt          http.RoundTripper
	accessToken string
}

func (t *accessTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer: "+t.accessToken)
	return t.rt.RoundTrip(req)
}

type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error represents the common errors returned by CockroacdhDB Cloud API
type Error struct {
	ErrorCode int
	HTTPCode  int
	Message   string
}

func (e *Error) Error() string {
	return e.Message
}
