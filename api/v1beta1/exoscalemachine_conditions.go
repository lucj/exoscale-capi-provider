package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// InstanceReadyCondition indicates that the Exoscale instance has been created and is running.
	InstanceReadyCondition clusterv1.ConditionType = "InstanceReady"
)

// GetConditions returns the conditions of the ExoscaleMachine.
func (m *ExoscaleMachine) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions on the ExoscaleMachine.
func (m *ExoscaleMachine) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}
