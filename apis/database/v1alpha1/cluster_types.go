/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"reflect"

	cockroachdb "github.com/cockroachdb/cockroach-cloud-sdk-go/pkg/client"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServerlessCluster struct {
	// +kubebuilder:validation:Required
	Regions []string `json:"regions"`
	// +optional
	// +kubebuilder:default=0
	SpendLimit *int32 `json:"spendLimit"`
}

// ClusterParameters are the configurable fields of a Cluster.
type ClusterParameters struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=CLOUD_PROVIDER_UNSPECIFIED;GCP;AWS
	Provider cockroachdb.ApiCloudProvider `json:"provider"`
	// +kubebuilder:validation:Required
	Serverless *ServerlessCluster `json:"serverless"`
}

// ClusterObservation are the observable fields of a Cluster.
type ClusterObservation struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cockroachdb}
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

func (c *Cluster) CreateClusterRequest() *cockroachdb.CreateClusterRequest {
	return &cockroachdb.CreateClusterRequest{
		Name:     c.Name,
		Provider: c.Spec.ForProvider.Provider,
		Spec: cockroachdb.CreateClusterSpecification{
			Serverless: &cockroachdb.ServerlessClusterCreateSpecification{
				Regions:    c.Spec.ForProvider.Serverless.Regions,
				SpendLimit: *c.Spec.ForProvider.Serverless.SpendLimit,
			},
		},
	}
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// Cluster type metadata.
var (
	ClusterKind             = reflect.TypeOf(Cluster{}).Name()
	ClusterGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterKind}.String()
	ClusterKindAPIVersion   = ClusterKind + "." + SchemeGroupVersion.String()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
