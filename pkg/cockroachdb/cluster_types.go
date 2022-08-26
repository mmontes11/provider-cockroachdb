package cockroachdb

import (
	"encoding/json"
	"fmt"
)

type Cluster struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Version  string    `json:"cockroachdb_version"`
	Plan     string    `json:"plan"`
	Provider *Provider `json:"cloud_provider"`
}

type Provider int

const (
	ProviderUnspecified Provider = iota
	ProviderGCP
	ProviderAWS
)

func (p *Provider) String() string {
	return map[Provider]string{
		ProviderUnspecified: "CLOUD_PROVIDER_UNSPECIFIED",
		ProviderGCP:         "GCP",
		ProviderAWS:         "AWS",
	}[*p]
}

func ProviderFromString(provider string) (Provider, error) {
	stringToProvider := map[string]Provider{
		"CLOUD_PROVIDER_UNSPECIFIED": ProviderUnspecified,
		"GCP":                        ProviderGCP,
		"AWS":                        ProviderAWS,
	}
	if p, ok := stringToProvider[provider]; ok {
		return p, nil
	}
	return 0, fmt.Errorf("provider '%s' not supported", provider)
}

func (p Provider) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *Provider) UnmarshallJSON(bytes []byte) error {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	provider, err := ProviderFromString(s)
	if err != nil {
		return err
	}
	*p = provider
	return nil
}
