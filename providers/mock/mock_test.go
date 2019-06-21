package mock

import (
	"context"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/commands/root"
	"gotest.tools/assert"
	"testing"
)

// Determine that annotations from the Provider are respected when constructing the Node
func TestNodeFromMockProvider(t *testing.T) {
	const (
		NodeName           = "nodeName"
		OperatingSystem    = "os"
		InternalIp         = "1.2.3.4"
		DaemonEndpointPort = 42
		Version            = "v0.1"
	)

	expectedAnnotations := map[string]string{
		"Key": "Value",
	}

	ctx := context.TODO()

	// Empty config
	mockConfig := MockConfig{}
	mockProvider, err := NewMockProviderMockConfig(mockConfig, NodeName, OperatingSystem, InternalIp, DaemonEndpointPort)
	assert.NilError(t, err)

	node := root.NodeFromProvider(ctx, NodeName, nil, mockProvider, Version)
	assert.Equal(t, 0, len(node.Annotations))

	// Annotated config
	mockConfig = MockConfig{
		Annotations: expectedAnnotations,
	}
	mockProvider, err = NewMockProviderMockConfig(mockConfig, NodeName, OperatingSystem, InternalIp, DaemonEndpointPort)
	assert.NilError(t, err)

	node = root.NodeFromProvider(ctx, NodeName, nil, mockProvider, Version)
	assert.DeepEqual(t, expectedAnnotations, node.Annotations)
}
