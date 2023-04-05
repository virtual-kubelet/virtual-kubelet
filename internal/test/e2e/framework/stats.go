package framework

import (
	"context"
	"encoding/json"

	api "github.com/virtual-kubelet/virtual-kubelet/node/api"
	stats "github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	"k8s.io/apimachinery/pkg/util/net"
)

// GetStatsSummary queries the /stats/summary endpoint of the virtual-kubelet and returns the Summary object obtained as a response.
func (f *Framework) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
	// Query the /stats/summary endpoint.
	b, err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Namespace(f.Namespace).
		Resource("pods").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("https", f.NodeName, "10250")).
		Suffix("/stats/summary").DoRaw(ctx)
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

// GetStatsSummary queries the /metrics/resource endpoint of the virtual-kubelet and returns the Summary object obtained as a response.
func (f *Framework) GetMetricsResource(ctx context.Context) ([]byte, error) {
	// Query the /stats/summary endpoint.
	b, err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Namespace(f.Namespace).
		Resource("pods").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("https", f.NodeName, "10250")).
		Suffix(api.MetricsResourceRouteSuffix).DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	return b, nil
}
