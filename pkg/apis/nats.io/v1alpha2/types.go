package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NatsCluster is a specification for a NatsCluster resource
//
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NatsCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the spec for a Nats cluster resource.
	Spec NatsClusterSpec `json:"spec"`
}

// NatsClusterTLSConfig is the optional TLS configuration for the cluster.
type NatsClusterTLSConfig struct {
	// ServerSecret is the secret containing the certificates
	// to secure the port to which the clients connect.
	ServerSecret string `json:"serverSecret,omitempty"`

	// RoutesSecret is the secret containing the certificates
	// to secure the port to which cluster routes connect.
	RoutesSecret string `json:"routesSecret,omitempty"`
}

// NatsClusterSpec is the spec for a single NATS cluster CRD.
type NatsClusterSpec struct {
	// Size is the number of NATS servers in a cluster.
	Size *int32 `json:"size"`

	// Version is the `gnatsd` release that the cluster
	// will be using.
	Version string `json:"version,omitempty"`

	// TLS is the configuration to secure the cluster.
	TLS *NatsClusterTLSConfig `json:"tls,omitempty"`
}

// NatsClusterList is a list of NatsCluster resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NatsClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NatsCluster `json:"items"`
}
