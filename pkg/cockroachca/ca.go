package cockroachca

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	cockroachdb "github.com/cockroachdb/cockroach-cloud-sdk-go/pkg/client"
)

const (
	defaultCAURL = "https://cockroachlabs.cloud/"
)

type CAOption func(*CAClient) error

func WithBaseURL(baseURL string) CAOption {
	return func(c *CAClient) error {
		url, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("error parsing base URL: %v", err)
		}
		c.baseURL = url

		return nil
	}
}

func WithHTTPClient(httpClient *http.Client) CAOption {
	return func(c *CAClient) error {
		c.httpClient = httpClient

		return nil
	}
}

type CAClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewCAClient(opts ...CAOption) (*CAClient, error) {
	url, err := url.Parse(defaultCAURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %v", err)
	}

	client := &CAClient{
		baseURL:    url,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("error setting option: %v", err)
		}
	}

	return client, nil
}

func (c *CAClient) ClusterCACert(ctx context.Context, cluster *cockroachdb.Cluster) ([]byte, error) {
	url, err := c.baseURL.Parse(fmt.Sprintf("https://cockroachlabs.cloud/clusters/%s/cert", cluster.Id))
	if err != nil {
		return nil, fmt.Errorf("error parsing CA cert URL: %v", err)
	}

	res, err := http.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("error requesting CA cert: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error requesting CA cert: status code %d", res.StatusCode)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert: %v", err)
	}
	return bytes, nil
}
