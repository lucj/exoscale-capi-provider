package controller

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type conditionSetter interface {
	GetConditions() clusterv1.Conditions
	SetConditions(clusterv1.Conditions)
}

// setCondition sets or updates a single condition on obj.  LastTransitionTime
// is only bumped when the status actually changes.
func setCondition(obj conditionSetter, condType clusterv1.ConditionType, status corev1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	conds := obj.GetConditions()
	for i, c := range conds {
		if c.Type == condType {
			if c.Status != status {
				conds[i].LastTransitionTime = now
			}
			conds[i].Status = status
			conds[i].Reason = reason
			conds[i].Message = message
			obj.SetConditions(conds)
			return
		}
	}
	conds = append(conds, clusterv1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
	obj.SetConditions(conds)
}
