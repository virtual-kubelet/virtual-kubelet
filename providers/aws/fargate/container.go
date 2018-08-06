package fargate

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container status strings.
	containerStatusProvisioning = "PROVISIONING"
	containerStatusPending      = "PENDING"
	containerStatusRunning      = "RUNNING"
	containerStatusStopped      = "STOPPED"

	// Container log configuration options.
	containerLogOptionRegion       = "awslogs-region"
	containerLogOptionGroup        = "awslogs-group"
	containerLogOptionStreamPrefix = "awslogs-stream-prefix"

	// Default container resource limits.
	containerDefaultCPULimit    int64 = VCPU / 4
	containerDefaultMemoryLimit int64 = 512 // * MiB
)

// Container is the representation of a Kubernetes container in Fargate.
type container struct {
	definition ecs.ContainerDefinition
	startTime  time.Time
	finishTime time.Time
}

// NewContainer creates a new container from a Kubernetes container spec.
func newContainer(spec *corev1.Container) (*container, error) {
	var cntr container

	// Translate the Kubernetes container spec to a Fargate container definition.
	cntr.definition = ecs.ContainerDefinition{
		Name:       aws.String(spec.Name),
		Image:      aws.String(spec.Image),
		EntryPoint: aws.StringSlice(spec.Command),
		Command:    aws.StringSlice(spec.Args),
	}

	if spec.WorkingDir != "" {
		cntr.definition.WorkingDirectory = aws.String(spec.WorkingDir)
	}

	// Add environment variables.
	if spec.Env != nil {
		for _, env := range spec.Env {
			cntr.definition.Environment = append(
				cntr.definition.Environment,
				&ecs.KeyValuePair{
					Name:  aws.String(env.Name),
					Value: aws.String(env.Value),
				})
		}
	}

	// Translate the Kubernetes container resource requirements to Fargate units.
	cntr.setResourceRequirements(&spec.Resources)

	return &cntr, nil
}

// NewContainerFromDefinition creates a new container from a Fargate container definition.
func newContainerFromDefinition(def *ecs.ContainerDefinition, startTime *time.Time) (*container, error) {
	var cntr container

	cntr.definition = *def

	if startTime != nil {
		cntr.startTime = *startTime
	}

	return &cntr, nil
}

// ConfigureLogs configures container logs to be sent to the given CloudWatch log group.
func (cntr *container) configureLogs(region string, logGroupName string, streamPrefix string) {
	streamPrefix = fmt.Sprintf("%s_%s", streamPrefix, *cntr.definition.Name)

	// Fargate requires awslogs log driver.
	cntr.definition.LogConfiguration = &ecs.LogConfiguration{
		LogDriver: aws.String(ecs.LogDriverAwslogs),
		Options: map[string]*string{
			containerLogOptionRegion:       aws.String(region),
			containerLogOptionGroup:        aws.String(logGroupName),
			containerLogOptionStreamPrefix: aws.String(streamPrefix),
		},
	}
}

// GetStatus returns the status of a container running in Fargate.
func (cntr *container) getStatus(runtimeState *ecs.Container) corev1.ContainerStatus {
	var reason string
	var state corev1.ContainerState
	var isReady bool

	if runtimeState.Reason != nil {
		reason = *runtimeState.Reason
	}

	switch *runtimeState.LastStatus {
	case containerStatusProvisioning,
		containerStatusPending:
		state = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason:  reason,
				Message: "",
			},
		}

	case containerStatusRunning:
		if cntr.startTime.IsZero() {
			cntr.startTime = time.Now()
		}

		isReady = true

		state = corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.NewTime(cntr.startTime),
			},
		}

	case containerStatusStopped:
		if cntr.finishTime.IsZero() {
			cntr.finishTime = time.Now()
		}

		var exitCode int32
		if runtimeState.ExitCode != nil {
			exitCode = int32(*runtimeState.ExitCode)
		}

		state = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				ExitCode:    exitCode,
				Signal:      0,
				Reason:      reason,
				Message:     "",
				StartedAt:   metav1.NewTime(cntr.startTime),
				FinishedAt:  metav1.NewTime(cntr.finishTime),
				ContainerID: "",
			},
		}
	}

	return corev1.ContainerStatus{
		Name:         *runtimeState.Name,
		State:        state,
		Ready:        isReady,
		RestartCount: 0,
		Image:        *cntr.definition.Image,
		ImageID:      "",
		ContainerID:  "",
	}
}

// SetResourceRequirements translates Kubernetes container resource requirements to Fargate units.
func (cntr *container) setResourceRequirements(reqs *corev1.ResourceRequirements) {
	//
	// Kubernetes container resource requirements consist of "limits" and "requests" for each
	// resource type. Limits are the maximum amount of resources allowed. Requests are the minimum
	// amount of resources reserved for the container. Both are optional. If requests are omitted,
	// they default to limits. If limits are also omitted, they both default to an
	// implementation-defined value.
	//
	// Fargate container resource requirements consist of CPU shares and memory limits. Memory is a
	// hard limit, which when exceeded, causes the container to be killed. MemoryReservation is a
	// the amount of resources reserved for the container. At least one must be specified.
	//

	// Use the defaults if the container does not have any resource requirements.
	cpu := containerDefaultCPULimit
	memory := containerDefaultMemoryLimit
	memoryReservation := containerDefaultMemoryLimit

	// Compute CPU requirements.
	if reqs != nil {
		var quantity resource.Quantity
		var ok bool

		// Fargate tasks do not share resources with other tasks. Therefore the task and each
		// container in it must be allocated their resource limits. Hence limits are preferred
		// over requests.
		if reqs.Limits != nil {
			quantity, ok = reqs.Limits[corev1.ResourceCPU]
		}
		if !ok && reqs.Requests != nil {
			quantity, ok = reqs.Requests[corev1.ResourceCPU]
		}
		if ok {
			// Because Fargate task CPU limit is the sum of the task's containers' CPU shares,
			// the container's CPU share equals its CPU limit.
			//
			// Convert CPU unit from Kubernetes milli-CPUs to EC2 vCPUs.
			cpu = quantity.ScaledValue(resource.Milli) * VCPU / 1000
		}
	}

	// Compute memory requirements.
	if reqs != nil {
		var reqQuantity resource.Quantity
		var limQuantity resource.Quantity
		var reqOk bool
		var limOk bool

		// Find the memory request and limit, if available.
		if reqs.Requests != nil {
			reqQuantity, reqOk = reqs.Requests[corev1.ResourceMemory]
		}
		if reqs.Limits != nil {
			limQuantity, limOk = reqs.Limits[corev1.ResourceMemory]
		}

		// If one is omitted, use the other one's value.
		if !limOk && reqOk {
			limQuantity = reqQuantity
		} else if !reqOk && limOk {
			reqQuantity = limQuantity
		}

		// If at least one is specified...
		if reqOk || limOk {
			// Convert memory unit from bytes to MiBs, rounding up to the next MiB.
			// This is necessary because Fargate container definition memory reservations and
			// limits are both in MiBs.
			memoryReservation = (reqQuantity.Value() + MiB - 1) / MiB
			memory = (limQuantity.Value() + MiB - 1) / MiB
		}
	}

	// Set final values.
	cntr.definition.Cpu = aws.Int64(cpu)
	cntr.definition.Memory = aws.Int64(memory)
	cntr.definition.MemoryReservation = aws.Int64(memoryReservation)
}
