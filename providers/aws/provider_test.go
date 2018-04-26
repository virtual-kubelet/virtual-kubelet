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
	vkAWS "github.com/virtual-kubelet/virtual-kubelet/providers/aws"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const testRegion = "us-east-1"

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

const testConfig = `
Region = "%s"
ClusterName = "%s"
CloudWatchLogGroupName = "%s"
ExecutionRoleArn = "%s"
Subnets = [ "%s" ]
SecurityGroups = [ ]
# Required to pull the busybox image
AssignPublicIPv4Address = true
`

const testName = "vk-aws-e2e-test"

func TestAWS(t *testing.T) {
	if os.Getenv("SKIP_AWS_E2E") == "1" {
		t.Skip("skipping AWS e2e tests")
	}

	session := session.New(&aws.Config{
		Region: aws.String(testRegion),
	})
	ec2Client := ec2.New(session)
	ecsClient := ecs.New(session)
	cloudwatchClient := cloudwatchlogs.New(session)
	iamClient := iam.New(session)

	// Create a new VPC with one subnet and internet access
	// Internet access is required to pull public images from the docker registry
	subnetID, err := createVpcWithInternetAccess(ec2Client)
	if err != nil {
		t.Error(err)
	}

	// Create the Cloudwatch Logs Log Group used by container logs
	logGroupdID := aws.String("/ecs/vk-aws-e2e-test")
	_, err = cloudwatchClient.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: logGroupdID,
	})
	if err != nil {
		t.Error(err)
	}

	// Create the role used by Fargate to write logs and pull ECR images
	executorRoleID := aws.String("vk-aws-e2e-test")
	_, err = iamClient.CreateRole(&iam.CreateRoleInput{
		RoleName:                 executorRoleID,
		AssumeRolePolicyDocument: aws.String(executorRoleAssumePolicy),
	})
	if err != nil {
		t.Error(err)
	}

	// Default policy allowing log writes and ECR pulls
	iamClient.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  executorRoleID,
	})
	if err != nil {
		t.Error(err)
	}

	// Create a cluster for the E2E test
	createResponse, err := ecsClient.CreateCluster(&ecs.CreateClusterInput{
		ClusterName: aws.String("vk-aws-e2e-test"),
	})
	if err != nil {
		t.Error(err)
	}
	clusterID := createResponse.Cluster.ClusterArn

	time.Sleep(10 * time.Second)

	t.Run("Create, list and delete pod", func(t *testing.T) {
		if clusterID == nil || logGroupdID == nil || executorRoleID == nil || subnetID == nil {
			t.Fatal("Can't start tests without all requirements.")
		}

		// Write config file with our test configuration
		config := fmt.Sprintf(testConfig, testRegion, "vk-aws-e2e-test", *logGroupdID, *executorRoleID, *subnetID)
		fmt.Printf("Test with config:\n%s", config)

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

		// Start the FargateProvider
		provider, err := vkAWS.NewFargateProvider(tmpfile.Name(), nil, "vk-aws-test", "Linux", "1.2.3.4", 10250)
		if err != nil {
			t.Fatal(err)
		}

		// Its a new cluster, there shouldn't be anything running
		pods, err := provider.GetPods()
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 0 {
			t.Errorf("Expect zero pods, but received %d pods\n%v", len(pods), pods)
		}

		// Create a test pod
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
						"-c", "echo \"Started\"; while true; do sleep 1; done",
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

		err = provider.CreatePod(pod)
		if err != nil {
			t.Fatal(err)
		}

		// Now there should be a pod running

		pods, err = provider.GetPods()
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 1 {
			t.Errorf("Expect one pods, but received %d pods\n%v", len(pods), pods)
		}

		// Wait until its scheduled and running
		err = waitUntilPodStatus(provider, podName, v1.PodRunning)
		if err != nil {
			t.Error(err)
		}

		// Some addition time for the logs to settle
		time.Sleep(10 * time.Second)

		logs, err := provider.GetContainerLogs("default", podName, "echo-container", 100)
		if err != nil {
			t.Error(err)
		}

		if logs != "Started\n" {
			t.Errorf("Expected logs to be \"Started\\n\", but received \"%v\"", logs)
		}

		// Remove the pod
		err = provider.DeletePod(pod)
		if err != nil {
			t.Fatal(err)
		}

		err = waitUntilPodStatus(provider, podName, v1.PodSucceeded)
		if err != nil {
			t.Error(err)
		}

		// The cluster should be empty again
		pods, err = provider.GetPods()
		if err != nil {
			t.Error(err)
		}
		if len(pods) != 0 {
			t.Errorf("Expect zero pods, but received %d pods\n%v", len(pods), pods)
		}
	})

	// Remove the cluster
	_, err = ecsClient.DeleteCluster(&ecs.DeleteClusterInput{
		Cluster: clusterID,
	})
	if err != nil {
		t.Error(err)
	}

	// Remove the execution role
	iamClient.DetachRolePolicy(&iam.DetachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"),
		RoleName:  executorRoleID,
	})
	if err != nil {
		t.Error(err)
	}

	_, err = iamClient.DeleteRole(&iam.DeleteRoleInput{
		RoleName: executorRoleID,
	})
	if err != nil {
		t.Error(err)
	}

	// Remove the log group
	_, err = cloudwatchClient.DeleteLogGroup(&cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: logGroupdID,
	})
	if err != nil {
		t.Error(err)
	}

	// Remove the VPC
	err = deleteVpc(ec2Client)
	if err != nil {
		t.Error(err)
	}
}

// waitUntilPodStatus polls pod status until the desired state was reached
func waitUntilPodStatus(provider *vkAWS.FargateProvider, podName string, desiredStatus v1.PodPhase) error {
	ctx := context.Background()
	context.WithTimeout(ctx, time.Duration(time.Second*60))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			status, err := provider.GetPodStatus("default", podName)
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
