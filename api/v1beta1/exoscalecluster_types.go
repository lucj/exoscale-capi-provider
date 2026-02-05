package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ExoscaleClusterSpec defines the desired state of ExoscaleCluster.
type ExoscaleClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// Set by the controller once the EIP is allocated.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// MasterSecurityGroup is the name of the security group to create (or reuse) for control-plane nodes.
	// +optional
	MasterSecurityGroup string `json:"masterSecurityGroup,omitempty"`

	// NodeSecurityGroup is the name of the security group to create (or reuse) for worker nodes.
	// +optional
	NodeSecurityGroup string `json:"nodeSecurityGroup,omitempty"`

	// Zone is the Exoscale zone where cluster resources will be created.
	Zone string `json:"zone"`
}

// ExoscaleClusterStatus defines the observed state of ExoscaleCluster.
type ExoscaleClusterStatus struct {
	// Ready indicates that the cluster is ready.
	Ready bool `json:"ready"`

	// ControlPlaneEndpoint is the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Conditions is a list of conditions for the ExoscaleCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// MasterSecurityGroupID is the ID of the security group used by control-plane nodes.
	// +optional
	MasterSecurityGroupID string `json:"masterSecurityGroupID,omitempty"`

	// NodeSecurityGroupID is the ID of the security group used by worker nodes.
	// +optional
	NodeSecurityGroupID string `json:"nodeSecurityGroupID,omitempty"`

	// EIPID is the ID of the Elastic IP allocated for the control plane endpoint.
	// +optional
	EIPID string `json:"eipID,omitempty"`

	// EIPAddress is the IP address of the Elastic IP.
	// +optional
	EIPAddress string `json:"eipAddress,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="ZONE",type="string",JSONPath=".spec.zone"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ExoscaleCluster is the Schema for the exoscaleclusters API.
type ExoscaleCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExoscaleClusterSpec   `json:"spec,omitempty"`
	Status ExoscaleClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExoscaleClusterList contains a list of ExoscaleCluster.
type ExoscaleClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExoscaleCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExoscaleCluster{}, &ExoscaleClusterList{})
}
