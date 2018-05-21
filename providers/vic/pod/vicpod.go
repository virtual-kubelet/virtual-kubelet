package pod

import (
	"k8s.io/api/core/v1"
)

type VicPod struct {
	ID  string
	Pod *v1.Pod
}
