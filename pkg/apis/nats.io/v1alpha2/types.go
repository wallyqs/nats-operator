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

// NatsClusterSpec is the spec for a Nats cluster resource.
type NatsClusterSpec struct {
	Nodes *int32 `json:"nodes"`
}

// NatsClusterList is a list of NatsCluster resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NatsClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NatsCluster `json:"items"`
}
