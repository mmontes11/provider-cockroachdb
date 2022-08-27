package cockroachdb

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Cluster struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   string    `json:"cockroachdb_version"`
	Plan      string    `json:"plan"`
	Provider  Provider  `json:"cloud_provider"`
	State     State     `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Provider string

const (
	ProviderUnspecified Provider = "CLOUD_PROVIDER_UNSPECIFIED"
	ProviderGCP         Provider = "GCP"
	ProviderAWS         Provider = "AWS"
)

type State string

const (
	StateUnspecified    State = "CLUSTER_STATE_UNSPECIFIED"
	StateCreating       State = "CREATING"
	StateCreated        State = "CREATED"
	StateCreationFailed State = "CREATION_FAILED"
	StateDeleted        State = "DELETED"
	StateLocker         State = "LOCKED"
)

type ClusterClient struct {
	client *Client
}
type ServerlessSpec struct {
	Regions    []string `json:"regions"`
	SpendLimit int      `json:"spend_limit"`
}

type ClusterSpec struct {
	Serverless ServerlessSpec `json:"serverless"`
}

type CreateCluster struct {
	Name     string       `json:"name"`
	Provider *Provider    `json:"provider"`
	Spec     *ClusterSpec `json:"spec"`
}

func (c *ClusterClient) Get(ctx context.Context, ID string) (*Cluster, error) {
	req, err := c.client.newRequest(http.MethodGet, fmt.Sprintf("/clusteers/%s", ID), nil)
	if err != nil {
		return nil, err
	}

	var cluster *Cluster
	if err := c.client.do(ctx, req, &cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

func (c *ClusterClient) Create(ctx context.Context, createCluster *CreateCluster) (*Cluster, error) {
	req, err := c.client.newRequest(http.MethodPost, "/clusters", createCluster)
	if err != nil {
		return nil, err
	}

	var cluster *Cluster
	if err := c.client.do(ctx, req, &cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
