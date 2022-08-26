package cockroachdb

import (
	"context"
	"net/http"
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
