package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ExoscaleClusterTemplateSpec defines the desired state of ExoscaleClusterTemplate.
type ExoscaleClusterTemplateSpec struct {
	Template ExoscaleClusterTemplateResource `json:"template"`
}

// ExoscaleClusterTemplateResource defines the template resource for ExoscaleCluster.
type ExoscaleClusterTemplateResource struct {
	Spec ExoscaleClusterTemplateResourceSpec `json:"spec"`
}

// ExoscaleClusterTemplateResourceSpec defines the user-settable cluster fields.
// ControlPlaneEndpoint is excluded because it is set by the controller.
type ExoscaleClusterTemplateResourceSpec struct {
	// MasterSecurityGroup is the name of the security group for control-plane nodes.
	// +optional
	MasterSecurityGroup string `json:"masterSecurityGroup,omitempty"`

	// NodeSecurityGroup is the name of the security group for worker nodes.
	// +optional
	NodeSecurityGroup string `json:"nodeSecurityGroup,omitempty"`

	// Zone is the Exoscale zone where cluster resources will be created.
	Zone string `json:"zone"`
}

// +kubebuilder:object:root=true

// ExoscaleClusterTemplate is the Schema for the exoscaleclustertemplates API.
type ExoscaleClusterTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ExoscaleClusterTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ExoscaleClusterTemplateList contains a list of ExoscaleClusterTemplate.
type ExoscaleClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExoscaleClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExoscaleClusterTemplate{}, &ExoscaleClusterTemplateList{})
}
