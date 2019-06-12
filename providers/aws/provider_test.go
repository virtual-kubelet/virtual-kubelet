package aws_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	vkAWS "github.com/virtual-kubelet/virtual-kubelet/providers/aws"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// E2E test configuration.
	testName          = "vk-fargate-e2e-test"
	defaultTestRegion = "us-east-1"

	// Environment variables that modify the test behavior.
	envSkipTests  = "SKIP_AWS_E2E"
	envTestRegion = "VK_TEST_FARGATE_REGION"
)

// executorRoleAssumePolicy is the policy used by task execution role.
const executorRoleAssumePolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
			"Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

// testConfig contains the Fargate provider test configuration template.
const testConfig = `
Region = "%s"
ClusterName = "%s"
Subnets = [ "%s" ]
SecurityGroups = [ ]
AssignPublicIPv4Address = true
ExecutionRoleArn = "%s"
CloudWatchLogGroupName = "%s"
`

var (
	ecsClient        *ecs.ECS
	testRegion       string
	subnetID         *string
	executorRoleName *string
	logGroupName     *string
)

// TestMain wraps the tests with the extra setup and teardown of AWS resources.
func TestMain(m *testing.M) {
	var err error

	// Skip the tests in this package if the environment variable is set.
	if os.Getenv(envSkipTests) == "1" {
		fmt.Println("Skipping AWS E2E tests.")
		os.Exit(0)
	}

	// Query the test region.
	region, ok := os.LookupEnv(envTestRegion)
	if ok {
		testRegion = region
	} else {
		testRegion = defaultTestRegion
	}
	fmt.Printf("Starting provider tests in region %s\n", testRegion)

	// Create the session and clients.
	session := session.New(&aws.Config{
		Region: aws.String(testRegion),
	})

	ecsClient = ecs.New(session)
	ec2Client := ec2.New(session)
	cloudwatchClient := cloudwatchlogs.New(session)
	iamClient := iam.New(session)

	// Create a test VPC with one subnet and internet access.
	// Internet access is required to pull public images from the docker registry.
	subnetID, err = createVpcWithInternetAccess(ec2Client)
	if err != nil {
		fmt.Printf("Failed to create VPC: %+v\n", err)
		os.Exit(-1)
	}

	// Create the AWS CloudWatch Logs log group used by containers.
	logGroupName = aws.String("/ecs/" + testName)
	_, err = cloudwatchClient.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: logGroupName,
	})
	if err != nil {
		fmt.Printf("Failed to create CloudWatch Logs log group: %+v\n", err)
		os.Exit(-1)
	}

	// Create the role used by Fargate to write logs and pull ECR images.
	executorRoleName = aws.String(testName)
	_, err = iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName:                 executorRoleName,
		AssumeRolePolicyDocument: aws.String(executorRoleAssumePolicy),
	})
	if err != nil {
		fmt.Printf("Failed to create task execution role: %+v", err)
		os.Exit(-1)
	}

	// Attach the default policy allowing log writes and ECR pulls.
	iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  executorRoleName,
	})
	if err != nil {
		fmt.Printf("Failed to attach role policy: %+v", err)
		os.Exit(-1)
	}

	// Run the tests.
	exitCode := m.Run()

	// Delete the task execution role.
	iamClient.DetachRolePolicy(&iam.DetachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  executorRoleName,
	})
	if err != nil {
		fmt.Printf("Failed to delete task execution role: %+v", err)
	}

	// Delete the role.
	_, err = iamClient.DeleteRole(&iam.DeleteRoleInput{
		RoleName: executorRoleName,
	})
	if err != nil {
		fmt.Printf("Failed to delete task execution role: %+v", err)
	}

	// Delete the log group.
	_, err = cloudwatchClient.DeleteLogGroup(&cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: logGroupName,
	})
	if err != nil {
		fmt.Printf("Failed to delete CloudWatch Logs log group: %+v\n", err)
	}

	// Delete the test VPC.
	err = deleteVpc(ec2Client)
	if err != nil {
		fmt.Printf("Failed to delete VPC: %+v\n", err)
	}

	os.Exit(exitCode)
}

// TestAWSFargateProviderPodLifecycle validates basic pod lifecycle by starting and stopping a pod.
func TestAWSFargateProviderPodLifecycle(t *testing.T) {
	// Create a cluster for the E2E test.
	createResponse, err := ecsClient.CreateCluster(&ecs.CreateClusterInput{
		ClusterName: aws.String(testName),
	})
	if err != nil {
		t.Error(err)
	}
	clusterID := createResponse.Cluster.ClusterArn

	time.Sleep(10 * time.Second)

	t.Run("Create, list and delete pod", func(t *testing.T) {
		// Write provider config file with test configuration.
		config := fmt.Sprintf(testConfig, testRegion, testName, *subnetID, *executorRoleName, *logGroupName)
		fmt.Printf("Fargate provider test configuration:%s", config)

		tmpfile, err := ioutil.TempFile("", "example")
		if err != nil {
			t.Fatal(err)
		}

		defer os.Remove(tmpfile.Name())

		if _, err = tmpfile.Write([]byte(config)); err != nil {
			t.Fatal(err)
		}

		if err = tmpfile.Close(); err != nil {
			t.Fatal(err)
		}

		// Start the Fargate provider.
		provider, err := vkAWS.NewFargateProvider(
			tmpfile.Name(), nil, testName, "Linux", "1.2.3.4", 10250)
		if err != nil {
			t.Fatal(err)
		}

		// Confirm that there are no pods on the cluster.
		pods, err := provider.GetPods(context.Background())
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 0 {
			t.Errorf("Expect zero pods, but received %d pods\n%v", len(pods), pods)
		}

		// Create a test pod.
		podName := fmt.Sprintf("test_%d", time.Now().UnixNano()/1000)

		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: "default",
				UID:       types.UID("unique"),
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{v1.Container{
					Name:  "echo-container",
					Image: "busybox",
					Command: []string{
						"/bin/sh",
					},
					Args: []string{
						"-c",
						"echo \"Started\";" +
							"echo \"TEST_ENV=$TEST_ENV\";" +
							"while true; do sleep 1; done",
					},
					Env: []v1.EnvVar{
						{Name: "TEST_ENV", Value: "AnyValue"},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("450Mi"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				}},
			},
		}

		err = provider.CreatePod(context.Background(), pod)
		if err != nil {
			t.Fatal(err)
		}

		// Now there should be exactly one pod.
		pods, err = provider.GetPods(context.Background())
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 1 {
			t.Errorf("Expect one pods, but received %d pods\n%v", len(pods), pods)
		}

		// Wait until the pod is running.
		err = waitUntilPodStatus(provider, podName, v1.PodRunning)
		if err != nil {
			t.Error(err)
		}

		// Wait a few seconds for the logs to settle.
		time.Sleep(10 * time.Second)

		logs, err := provider.GetContainerLogs(context.Background(), "default", podName, "echo-container", api.ContainerLogOpts{Tail: 100})
		if err != nil {
			t.Fatal(err)
		}
		defer logs.Close()

		b, err := ioutil.ReadAll(logs)
		if err != nil {
			t.Fatal(err)
		}

		// Test log output.
		receivedLogs := strings.Split(string(b), "\n")
		expectedLogs := []string{
			"Started",
			pod.Spec.Containers[0].Env[0].Name + "=" + pod.Spec.Containers[0].Env[0].Value,
		}

		for i, line := range receivedLogs {
			fmt.Printf("Log[#%d]: %v\n", i, line)
			if len(expectedLogs) > i && receivedLogs[i] != expectedLogs[i] {
				t.Errorf("Expected log line %d to be %q, but received %q", i, line, receivedLogs[i])
			}
		}

		// Delete the pod.
		err = provider.DeletePod(context.Background(), pod)
		if err != nil {
			t.Fatal(err)
		}

		err = waitUntilPodStatus(provider, podName, v1.PodSucceeded)
		if err != nil {
			t.Error(err)
		}

		// The cluster should be empty again.
		pods, err = provider.GetPods(context.Background())
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 0 {
			t.Errorf("Expect zero pods, but received %d pods\n%v", len(pods), pods)
		}
	})

	// Delete the test cluster.
	_, err = ecsClient.DeleteCluster(&ecs.DeleteClusterInput{
		Cluster: clusterID,
	})
	if err != nil {
		t.Error(err)
	}
}

// waitUntilPodStatus polls pod status until the desired state is reached.
func waitUntilPodStatus(provider *vkAWS.FargateProvider, podName string, desiredStatus v1.PodPhase) error {
	ctx := context.Background()
	context.WithTimeout(ctx, time.Duration(time.Second*60))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			status, err := provider.GetPodStatus(context.Background(), "default", podName)
			if err != nil {
				if strings.Contains(err.Error(), "is not found") {
					return nil
				}

				return err
			}
			if status.Phase == desiredStatus {
				return nil
			}

			time.Sleep(3 * time.Second)
		}
	}
}
