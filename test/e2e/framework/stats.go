package framework

import (
	"encoding/json"
	"strconv"

	"k8s.io/apimachinery/pkg/util/net"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// GetStatsSummary queries the /stats/summary endpoint of the virtual-kubelet and returns the Summary object obtained as a response.
func (f *Framework) GetStatsSummary() (*stats.Summary, error) {
	// Query the /stats/summary endpoint.
	b, err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Namespace(f.Namespace).
		Resource("pods").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", f.NodeName, strconv.Itoa(10255))).
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
