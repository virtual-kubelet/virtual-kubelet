package nodeutil

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetNodeReady sets the node ready condition to true
func SetNodeReady(n *corev1.Node) {
	for i, c := range n.Status.Conditions {
		if c.Type != "Ready" {
			continue
		}

		c.Message = "Kubelet is ready"
		c.Reason = "KubeletReady"
		c.Status = corev1.ConditionTrue
		c.LastHeartbeatTime = metav1.Now()
		c.LastTransitionTime = metav1.Now()
		n.Status.Conditions[i] = c
		return
	}

	// No ready condition in node status
	c := corev1.NodeCondition{
		Type:               "Ready",
		Status:             corev1.ConditionTrue,
		Reason:             "KubeletReady",
		Message:            "Kubelet is ready",
		LastHeartbeatTime:  metav1.Now(),
		LastTransitionTime: metav1.Now(),
	}
	n.Status.Conditions = append(n.Status.Conditions, c)
}
