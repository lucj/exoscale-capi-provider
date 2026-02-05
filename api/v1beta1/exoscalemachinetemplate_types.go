package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ExoscaleMachineTemplateSpec defines the desired state of ExoscaleMachineTemplate.
type ExoscaleMachineTemplateSpec struct {
	Template ExoscaleMachineTemplateResource `json:"template"`
}

// ExoscaleMachineTemplateResource defines the template resource for ExoscaleMachine.
type ExoscaleMachineTemplateResource struct {
	Spec ExoscaleMachineTemplateResourceSpec `json:"spec"`
}

// ExoscaleMachineTemplateResourceSpec defines the user-settable machine fields.
// ProviderID is excluded because it is set by the controller.
type ExoscaleMachineTemplateResourceSpec struct {
	// Zone is the Exoscale zone where the instance will be created.
	Zone string `json:"zone"`

	// Template is the name or ID of the Exoscale compute instance template.
	Template string `json:"template"`

	// InstanceType is the name or ID of the Exoscale instance type.
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

// +kubebuilder:object:root=true

// ExoscaleMachineTemplate is the Schema for the exoscalemachinettemplates API.
type ExoscaleMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ExoscaleMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ExoscaleMachineTemplateList contains a list of ExoscaleMachineTemplate.
type ExoscaleMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExoscaleMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExoscaleMachineTemplate{}, &ExoscaleMachineTemplateList{})
}
