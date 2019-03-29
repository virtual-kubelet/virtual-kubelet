package fargate

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	anyCPURequest    = "500m"
	anyCPULimit      = "2"
	anyMemoryRequest = "768Mi"
	anyMemoryLimit   = "2Gi"

	anyContainerName     = "any container name"
	anyContainerImage    = "any container image"
	anyContainerReason   = "any reason"
	anyContainerExitCode = 42
)

var (
	anyContainerSpec = corev1.Container{
		Name:       anyContainerName,
		Image:      anyContainerImage,
		Command:    []string{"anyCmd"},
		Args:       []string{"anyArg1", "anyArg2"},
		WorkingDir: "/any/working/dir",
		Env: []corev1.EnvVar{
			{Name: "anyEnvName1", Value: "anyEnvValue1"},
			{Name: "anyEnvName2", Value: "anyEnvValue2"},
		},
	}
)

// TestCreateContainer verifies whether Kubernetes container specs are translated to
// Fargate container definitions correctly.
func TestContainerDefinition(t *testing.T) {
	cntrSpec := anyContainerSpec

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	assert.Check(t, is.Equal(cntrSpec.Name, *cntr.definition.Name), "incorrect name")
	assert.Check(t, is.Equal(cntrSpec.Image, *cntr.definition.Image), "incorrect image")
	assert.Check(t, is.Equal(cntrSpec.Command[0], *cntr.definition.EntryPoint[0]), "incorrect command")

	for i, env := range cntrSpec.Args {
		assert.Check(t, is.Equal(env, *cntr.definition.Command[i]), "incorrect args")
	}

	assert.Check(t, is.Equal(cntrSpec.WorkingDir, *cntr.definition.WorkingDirectory), "incorrect working dir")

	for i, env := range cntrSpec.Env {
		assert.Check(t, is.Equal(env.Name, *cntr.definition.Environment[i].Name), "incorrect env name")
		assert.Check(t, is.Equal(env.Value, *cntr.definition.Environment[i].Value), "incorrect env value")
	}
}

// TestContainerResourceRequirementsDefaults verifies whether the container gets default CPU
// and memory resources when none is specified.
func TestContainerResourceRequirementsDefaults(t *testing.T) {
	cntrSpec := anyContainerSpec

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	assert.Check(t, is.Equal(containerDefaultCPULimit, *cntr.definition.Cpu), "incorrect CPU limit")
	assert.Check(t, is.Equal(containerDefaultMemoryLimit, *cntr.definition.Memory), "incorrect memory limit")
}

// TestContainerResourceRequirementsWithRequestsNoLimits verifies whether the container gets
// correct CPU and memory requests when only requests are specified.
func TestContainerResourceRequirementsWithRequestsNoLimits(t *testing.T) {
	cntrSpec := anyContainerSpec
	cntrSpec.Resources = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(anyCPURequest),
			corev1.ResourceMemory: resource.MustParse(anyMemoryRequest),
		},
	}

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	assert.Check(t, is.Equal(int64(512), *cntr.definition.Cpu), "incorrect CPU limit")
	assert.Check(t, is.Equal(int64(768), *cntr.definition.Memory), "incorrect memory limit")
}

// TestContainerResourceRequirementsWithLimitsNoRequests verifies whether the container gets
// correct CPU and memory limits when only limits are specified.
func TestContainerResourceRequirementsWithLimitsNoRequests(t *testing.T) {
	cntrSpec := anyContainerSpec
	cntrSpec.Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(anyCPULimit),
			corev1.ResourceMemory: resource.MustParse(anyMemoryLimit),
		},
	}

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	assert.Check(t, is.Equal(int64(2048), *cntr.definition.Cpu), "incorrect CPU limit")
	assert.Check(t, is.Equal(int64(2048), *cntr.definition.Memory), "incorrect memory limit")
}

// TestContainerResourceRequirementsWithRequestsAndLimits verifies whether the container gets
// correct CPU and memory limits when both requests and limits are specified.
func TestContainerResourceRequirementsWithRequestsAndLimits(t *testing.T) {
	cntrSpec := anyContainerSpec
	cntrSpec.Resources = corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(anyCPURequest),
			corev1.ResourceMemory: resource.MustParse(anyMemoryRequest),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse(anyCPULimit),
			corev1.ResourceMemory: resource.MustParse(anyMemoryLimit),
		},
	}

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	assert.Check(t, is.Equal(int64(2048), *cntr.definition.Cpu), "incorrect CPU limit")
	assert.Check(t, is.Equal(int64(2048), *cntr.definition.Memory), "incorrect memory limit")
}

// TestContainerResourceRequirements verifies whether Kubernetes container resource requirements
// are translated to Fargate container resource requests correctly.
func TestContainerResourceRequirementsTranslations(t *testing.T) {
	type testCase struct {
		requestedCPU         string
		requestedMemory      string
		expectedCPU          int64
		expectedMemoryInMiBs int64
	}

	// Expected and observed CPU quantities are in units of 1/1024th vCPUs.
	var testCases = []testCase{
		// Missing or partial resource requests.
		{"", "", 256, 512},
		{"100m", "", 102, 512},
		{"", "256Mi", 256, 256},

		// Minimum CPU request.
		{"1m", "1Mi", 1, 1},

		// Small memory request rounded up to the next MiB.
		{"250m", "1Ki", 256, 1},
		{"250m", "100Ki", 256, 1},
		{"250m", "500Ki", 256, 1},
		{"250m", "1024Ki", 256, 1},
		{"250m", "1025Ki", 256, 2},

		// Common combinations.
		{"200m", "300Mi", 204, 300},
		{"500m", "500Mi", 512, 500},
		{"1000m", "512Mi", 1024, 512},
		{"1", "512Mi", 1024, 512},
		{"1500m", "1000Mi", 1536, 1000},
		{"1500m", "1024Mi", 1536, 1024},
		{"2", "2Gi", 2048, 2048},
		{"4", "30Gi", 4096, 30 * 1024},

		// Very large requests.
		{"8", "42Gi", 8192, 42 * 1024},
		{"10", "128Gi", 10240, 128 * 1024},
	}

	for _, tc := range testCases {
		t.Run(
			fmt.Sprintf("cpu:%s,memory:%s", tc.requestedCPU, tc.requestedMemory),
			func(t *testing.T) {
				reqs := corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{},
				}

				if tc.requestedCPU != "" {
					reqs.Limits[corev1.ResourceCPU] = resource.MustParse(tc.requestedCPU)
				}

				if tc.requestedMemory != "" {
					reqs.Limits[corev1.ResourceMemory] = resource.MustParse(tc.requestedMemory)
				}

				cntrSpec := anyContainerSpec
				cntrSpec.Resources = reqs

				cntr, err := newContainer(&cntrSpec)
				assert.NilError(t, err, "failed to create container")

				assert.Check(t,
					*cntr.definition.Cpu == tc.expectedCPU && *cntr.definition.Memory == tc.expectedMemoryInMiBs,
					"requested (cpu:%v memory:%v) expected (cpu:%v memory:%v) observed (cpu:%v memory:%v)",
					tc.requestedCPU, tc.requestedMemory,
					tc.expectedCPU, tc.expectedMemoryInMiBs,
					*cntr.definition.Cpu, *cntr.definition.Memory)
			})
	}
}

// TestContainerStatus verifies whether Kubernetes containers report their status correctly for
// all Fargate container state transitions.
func TestContainerStatus(t *testing.T) {
	cntrSpec := anyContainerSpec

	cntr, err := newContainer(&cntrSpec)
	assert.NilError(t, err, "failed to create container")

	// Fargate container status provisioning.
	state := ecs.Container{
		Name:       aws.String(anyContainerName),
		Reason:     aws.String(anyContainerReason),
		LastStatus: aws.String(containerStatusProvisioning),
		ExitCode:   aws.Int64(0),
	}

	status := cntr.getStatus(&state)

	assert.Check(t, is.Equal(anyContainerName, status.Name), "incorrect name")
	assert.Check(t, status.State.Waiting != nil, "incorrect state")
	assert.Check(t, is.Equal(anyContainerReason, status.State.Waiting.Reason), "incorrect reason")
	assert.Check(t, is.Nil(status.State.Running), "incorrect state")
	assert.Check(t, is.Nil(status.State.Terminated), "incorrect state")
	assert.Check(t, !status.Ready, "incorrect ready")
	assert.Check(t, is.Equal(anyContainerImage, status.Image), "incorrect image")

	// Fargate container status pending.
	state.LastStatus = aws.String(containerStatusPending)
	status = cntr.getStatus(&state)

	assert.Check(t, is.Equal(anyContainerName, status.Name), "incorrect name")
	assert.Check(t, status.State.Waiting != nil, "incorrect state")
	assert.Check(t, is.Equal(anyContainerReason, status.State.Waiting.Reason), "incorrect reason")
	assert.Check(t, is.Nil(status.State.Running), "incorrect state")
	assert.Check(t, is.Nil(status.State.Terminated), "incorrect state")
	assert.Check(t, !status.Ready, "incorrect ready")
	assert.Check(t, is.Equal(anyContainerImage, status.Image), "incorrect image")

	// Fargate container status running.
	state.LastStatus = aws.String(containerStatusRunning)
	status = cntr.getStatus(&state)

	assert.Check(t, is.Equal(anyContainerName, status.Name), "incorrect name")
	assert.Check(t, is.Nil(status.State.Waiting), "incorrect state")
	assert.Check(t, status.State.Running != nil, "incorrect state")
	assert.Check(t, !status.State.Running.StartedAt.IsZero(), "incorrect startedat")
	assert.Check(t, is.Nil(status.State.Terminated), "incorrect state")
	assert.Check(t, status.Ready, "incorrect ready")
	assert.Check(t, is.Equal(anyContainerImage, status.Image), "incorrect image")

	// Fargate container status stopped.
	state.LastStatus = aws.String(containerStatusStopped)
	state.ExitCode = aws.Int64(anyContainerExitCode)
	status = cntr.getStatus(&state)

	assert.Check(t, is.Equal(anyContainerName, status.Name), "incorrect name")
	assert.Check(t, is.Nil(status.State.Waiting), "incorrect state")
	assert.Check(t, is.Nil(status.State.Running), "incorrect state")
	assert.Check(t, status.State.Terminated != nil, "incorrect state")
	assert.Check(t, is.Equal(int32(anyContainerExitCode), status.State.Terminated.ExitCode), "incorrect exitcode")
	assert.Check(t, is.Equal(anyContainerReason, status.State.Terminated.Reason), "incorrect reason")
	assert.Check(t, !status.State.Terminated.StartedAt.IsZero(), "incorrect startedat")
	assert.Check(t, !status.State.Terminated.FinishedAt.IsZero(), "incorrect finishedat")
	assert.Check(t, !status.Ready, "incorrect ready")
	assert.Check(t, is.Equal(anyContainerImage, status.Image), "incorrect image")
}
