package framework

import (
	"encoding/json"
	"strconv"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func getMetricsPort(pod *v1.Pod) string {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "metrics" {
				return strconv.Itoa(int(port.HostPort))
			}
		}
	}
	return ""
}

// GetStatsSummary queries the /stats/summary endpoint of the virtual-kubelet and returns the Summary object obtained as a response.
func (f *Framework) GetStatsSummary() (*stats.Summary, error) {
	kubeletPod, err := f.KubeClient.CoreV1().Pods(f.Namespace).Get(f.NodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	metricsPort := "10255"
	if port := getMetricsPort(kubeletPod); port != "" {
		metricsPort = port
	}
	// Query the /stats/summary endpoint.
	b, err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Namespace(f.Namespace).
		Resource("pods").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", f.NodeName, metricsPort)).
		Suffix("/stats/summary").DoRaw()
	if err != nil {
		return nil, err
	}
	// Unmarshal the response as a Summary object and return it.
	res := &stats.Summary{}
	if err := json.Unmarshal(b, res); err != nil {
		return nil, err
	}
	return res, nil
}
