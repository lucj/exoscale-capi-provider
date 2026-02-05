package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ExoscaleMachineSpec defines the desired state of ExoscaleMachine.
type ExoscaleMachineSpec struct {
	// ProviderID is the unique identifier of the instance in the format exoscale://<uuid>.
	// Set by the controller after the instance is created.
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Zone is the Exoscale zone where the instance will be created.
	Zone string `json:"zone"`

	// Template is the name or ID of the Exoscale compute instance template.
	Template string `json:"template"`

	// InstanceType is the name or ID of the Exoscale instance type (e.g. "standard.medium").
	InstanceType string `json:"instanceType"`

	// DiskSize is the size of the instance disk in GB.
	DiskSize int64 `json:"diskSize"`

	// SSHKey is the name of the SSH key pair to use for the instance.
	SSHKey string `json:"sshKey"`

	// AntiAffinityGroup is the optional name or ID of an anti-affinity group.
	// +optional
	AntiAffinityGroup string `json:"antiAffinityGroup,omitempty"`

	// IPv6 enables IPv6 networking on the instance.
	// +optional
	IPv6 bool `json:"ipv6,omitempty"`
}

// ExoscaleMachineStatus defines the observed state of ExoscaleMachine.
type ExoscaleMachineStatus struct {
	// Ready indicates that the machine is ready.
	Ready bool `json:"ready"`

	// Addresses is a list of addresses associated with the instance.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// InstanceID is the ID of the Exoscale instance.
	// +optional
	InstanceID string `json:"instanceID,omitempty"`

	// Conditions is a list of conditions for the ExoscaleMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="INSTANCE",type="string",JSONPath=".status.instanceID"
// +kubebuilder:printcolumn:name="ZONE",type="string",JSONPath=".spec.zone"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ExoscaleMachine is the Schema for the exoscalemachines API.
type ExoscaleMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExoscaleMachineSpec   `json:"spec,omitempty"`
	Status ExoscaleMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExoscaleMachineList contains a list of ExoscaleMachine.
type ExoscaleMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExoscaleMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExoscaleMachine{}, &ExoscaleMachineList{})
}
