package vkubelet

import (
	"context"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// registerNode registers this virtual node with the Kubernetes API.
func (s *Server) registerNode(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "registerNode")
	defer span.End()

	taints := make([]corev1.Taint, 0)

	if s.taint != nil {
		taints = append(taints, *s.taint)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.nodeName,
			Labels: map[string]string{
				"type":                                                    "virtual-kubelet",
				"kubernetes.io/role":                                      "agent",
				"beta.kubernetes.io/os":                                   strings.ToLower(s.provider.OperatingSystem()),
				"kubernetes.io/hostname":                                  s.nodeName,
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: s.provider.OperatingSystem(),
				Architecture:    "amd64",
				KubeletVersion:  "v1.12.2",
			},
			Capacity:        s.provider.Capacity(ctx),
			Allocatable:     s.provider.Capacity(ctx),
			Conditions:      s.provider.NodeConditions(ctx),
			Addresses:       s.provider.NodeAddresses(ctx),
			DaemonEndpoints: *s.provider.NodeDaemonEndpoints(ctx),
		},
	}
	addNodeAttributes(span, node)
	if _, err := s.k8sClient.CoreV1().Nodes().Create(node); err != nil && !errors.IsAlreadyExists(err) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return err
	}
	span.Annotate(nil, "Registered node with k8s")

	log.G(ctx).Info("Registered node")

	return nil
}

// updateNode updates the node status within Kubernetes with updated NodeConditions.
func (s *Server) updateNode(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updateNode")
	defer span.End()

	opts := metav1.GetOptions{}
	n, err := s.k8sClient.CoreV1().Nodes().Get(s.nodeName, opts)
	if err != nil && !errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Error("Failed to retrieve node")
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return
	}
	addNodeAttributes(span, n)
	span.Annotate(nil, "Fetched node details from k8s")

	if errors.IsNotFound(err) {
		if err = s.registerNode(ctx); err != nil {
			log.G(ctx).WithError(err).Error("Failed to register node")
			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		} else {
			span.Annotate(nil, "Registered node in k8s")
		}
		return
	}

	n.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	n.Status.Conditions = s.provider.NodeConditions(ctx)

	capacity := s.provider.Capacity(ctx)
	n.Status.Capacity = capacity
	n.Status.Allocatable = capacity

	n.Status.Addresses = s.provider.NodeAddresses(ctx)

	n, err = s.k8sClient.CoreV1().Nodes().UpdateStatus(n)
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to update node")
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return
	}
}

type taintsStringer []corev1.Taint

func (t taintsStringer) String() string {
	var s string
	for _, taint := range t {
		if s == "" {
			s = taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		} else {
			s += ", " + taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		}
	}
	return s
}

func addNodeAttributes(span *trace.Span, n *corev1.Node) {
	span.AddAttributes(
		trace.StringAttribute("UID", string(n.UID)),
		trace.StringAttribute("name", n.Name),
		trace.StringAttribute("cluster", n.ClusterName),
	)
	if span.IsRecordingEvents() {
		span.AddAttributes(trace.StringAttribute("taints", taintsStringer(n.Spec.Taints).String()))
	}
}
