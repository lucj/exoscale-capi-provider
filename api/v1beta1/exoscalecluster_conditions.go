package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// SecurityGroupsReadyCondition indicates that the security groups have been created and are ready.
	SecurityGroupsReadyCondition clusterv1.ConditionType = "SecurityGroupsReady"

	// EIPReadyCondition indicates that the Elastic IP has been allocated and is ready.
	EIPReadyCondition clusterv1.ConditionType = "EIPReady"
)

// GetConditions returns the conditions of the ExoscaleCluster.
func (c *ExoscaleCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on the ExoscaleCluster.
func (c *ExoscaleCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}
