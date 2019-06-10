package fargate

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

const (
	clusterFailureReasonMissing = "MISSING"
)

// ClusterConfig contains a Fargate cluster's configurable parameters.
type ClusterConfig struct {
	Region                  string
	Name                    string
	NodeName                string
	Subnets                 []string
	SecurityGroups          []string
	AssignPublicIPv4Address bool
	ExecutionRoleArn        string
	CloudWatchLogGroupName  string
	PlatformVersion         string
}

// Cluster represents a Fargate cluster.
type Cluster struct {
	region                  string
	name                    string
	nodeName                string
	arn                     string
	subnets                 []string
	securityGroups          []string
	assignPublicIPv4Address bool
	executionRoleArn        string
	cloudWatchLogGroupName  string
	platformVersion         string
	pods                    map[string]*Pod
	sync.RWMutex
}

// NewCluster creates a new Cluster object.
func NewCluster(config *ClusterConfig) (*Cluster, error) {
	var err error

	// Cluster name cannot contain '_' as it is used as a separator in task tags.
	if strings.Contains(config.Name, "_") {
		return nil, fmt.Errorf("cluster name should not contain the '_' character")
	}

	// Check if Fargate is available in the given region.
	if !FargateRegions.Include(config.Region) {
		return nil, fmt.Errorf("Fargate is not available in region %s", config.Region)
	}

	// Create the client to the regional Fargate service.
	client, err = newClient(config.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create Fargate client: %v", err)
	}

	// Initialize the cluster.
	cluster := &Cluster{
		region:                  config.Region,
		name:                    config.Name,
		nodeName:                config.NodeName,
		subnets:                 config.Subnets,
		securityGroups:          config.SecurityGroups,
		assignPublicIPv4Address: config.AssignPublicIPv4Address,
		executionRoleArn:        config.ExecutionRoleArn,
		cloudWatchLogGroupName:  config.CloudWatchLogGroupName,
		platformVersion:         config.PlatformVersion,
		pods:                    make(map[string]*Pod),
	}

	// If a node name is not specified, use the Fargate cluster name.
	if cluster.nodeName == "" {
		cluster.nodeName = cluster.name
	}

	// Check if the cluster already exists.
	err = cluster.describe()
	if err != nil && !strings.Contains(err.Error(), clusterFailureReasonMissing) {
		return nil, err
	}

	// If not, try to create it.
	// This might fail if the principal doesn't have the necessary permission.
	if cluster.arn == "" {
		err = cluster.create()
		if err != nil {
			return nil, err
		}
	}

	// Load existing pod state from Fargate to the local cache.
	err = cluster.loadPodState()
	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// Create creates a new Fargate cluster.
func (c *Cluster) create() error {
	api := client.api

	input := &ecs.CreateClusterInput{
		ClusterName: aws.String(c.name),
	}

	log.Printf("Creating Fargate cluster %s in region %s", c.name, c.region)

	output, err := api.CreateCluster(input)
	if err != nil {
		err = fmt.Errorf("failed to create cluster: %v", err)
		log.Println(err)
		return err
	}

	c.arn = aws.StringValue(output.Cluster.ClusterArn)
	log.Printf("Created Fargate cluster %s in region %s", c.name, c.region)

	return nil
}

// Describe loads information from an existing Fargate cluster.
func (c *Cluster) describe() error {
	api := client.api

	input := &ecs.DescribeClustersInput{
		Clusters: aws.StringSlice([]string{c.name}),
	}

	log.Printf("Looking for Fargate cluster %s in region %s.", c.name, c.region)

	output, err := api.DescribeClusters(input)
	if err != nil || len(output.Clusters) == 0 {
		if len(output.Failures) > 0 {
			err = fmt.Errorf("reason: %s", *output.Failures[0].Reason)
		}
		err = fmt.Errorf("failed to describe cluster: %v", err)
		log.Println(err)
		return err
	}

	log.Printf("Found Fargate cluster %s in region %s.", c.name, c.region)
	c.arn = aws.StringValue(output.Clusters[0].ClusterArn)

	return nil
}

// LoadPodState rebuilds pod and container objects in this cluster by loading existing tasks from
// Fargate. This is done during startup and whenever the local state is suspected to be out of sync
// with the actual state in Fargate. Caching state locally minimizes the number of service calls.
func (c *Cluster) loadPodState() error {
	api := client.api

	log.Printf("Loading pod state from cluster %s.", c.name)

	taskArns := make([]*string, 0)

	// Get a list of all Fargate tasks running on this cluster.
	err := api.ListTasksPages(
		&ecs.ListTasksInput{
			Cluster:       aws.String(c.name),
			DesiredStatus: aws.String(ecs.DesiredStatusRunning),
			LaunchType:    aws.String(ecs.LaunchTypeFargate),
		},
		func(page *ecs.ListTasksOutput, lastPage bool) bool {
			taskArns = append(taskArns, page.TaskArns...)
			return !lastPage
		},
	)

	if err != nil {
		err := fmt.Errorf("failed to load pod state: %v", err)
		log.Println(err)
		return err
	}

	log.Printf("Found %d tasks on cluster %s.", len(taskArns), c.name)

	pods := make(map[string]*Pod)

	// For each task running on this Fargate cluster...
	for _, taskArn := range taskArns {
		// Describe the task.
		describeTasksOutput, err := api.DescribeTasks(
			&ecs.DescribeTasksInput{
				Cluster: aws.String(c.name),
				Tasks:   []*string{taskArn},
			},
		)

		if err != nil || len(describeTasksOutput.Tasks) != 1 {
			log.Printf("Failed to describe task %s. Skipping.", *taskArn)
			continue
		}

		task := describeTasksOutput.Tasks[0]

		// Describe the task definition.
		describeTaskDefinitionOutput, err := api.DescribeTaskDefinition(
			&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: task.TaskDefinitionArn,
			},
		)

		if err != nil {
			log.Printf("Failed to describe task definition %s. Skipping.", *task.TaskDefinitionArn)
			continue
		}

		taskDef := describeTaskDefinitionOutput.TaskDefinition

		// A pod's tag is stored in its task definition's Family field.
		tag := aws.StringValue(taskDef.Family)

		// Rebuild the pod object.
		// Not all tasks are necessarily pods. Skip tasks that do not have a valid tag.
		pod, err := NewPodFromTag(c, tag)
		if err != nil {
			log.Printf("Skipping unknown task %s: %v", *taskArn, err)
			continue
		}

		pod.uid = k8sTypes.UID(aws.StringValue(task.StartedBy))
		pod.taskDefArn = aws.StringValue(task.TaskDefinitionArn)
		pod.taskArn = aws.StringValue(task.TaskArn)
		if taskDef.TaskRoleArn != nil {
			pod.taskRoleArn = aws.StringValue(taskDef.TaskRoleArn)
		}
		pod.taskStatus = aws.StringValue(task.LastStatus)
		pod.taskRefreshTime = time.Now()

		// Rebuild the container objects.
		for _, cntrDef := range taskDef.ContainerDefinitions {
			cntr, _ := newContainerFromDefinition(cntrDef, task.CreatedAt)

			pod.taskCPU += aws.Int64Value(cntr.definition.Cpu)
			pod.taskMemory += aws.Int64Value(cntr.definition.Memory)
			pod.containers[aws.StringValue(cntrDef.Name)] = cntr

			log.Printf("Found pod %s/%s on cluster %s.", pod.namespace, pod.name, c.name)
		}

		pods[tag] = pod
	}

	// Update local state.
	c.Lock()
	c.pods = pods
	c.Unlock()

	return nil
}

// GetPod returns a Kubernetes pod deployed on this cluster.
func (c *Cluster) GetPod(namespace string, name string) (*Pod, error) {
	c.RLock()
	defer c.RUnlock()

	tag := buildTaskDefinitionTag(c.name, namespace, name)
	pod, ok := c.pods[tag]
	if !ok {
		return nil, errdefs.NotFoundf("pod %s/%s is not found", namespace, name)
	}

	return pod, nil
}

// GetPods returns all Kubernetes pods deployed on this cluster.
func (c *Cluster) GetPods() ([]*Pod, error) {
	c.RLock()
	defer c.RUnlock()

	pods := make([]*Pod, 0, len(c.pods))

	for _, pod := range c.pods {
		pods = append(pods, pod)
	}

	return pods, nil
}

// InsertPod inserts a Kubernetes pod to this cluster.
func (c *Cluster) InsertPod(pod *Pod, tag string) {
	c.Lock()
	defer c.Unlock()

	c.pods[tag] = pod
}

// RemovePod removes a Kubernetes pod from this cluster.
func (c *Cluster) RemovePod(tag string) {
	c.Lock()
	defer c.Unlock()

	delete(c.pods, tag)
}

// GetContainerLogs returns the logs of a container from this cluster.
func (c *Cluster) GetContainerLogs(namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	if c.cloudWatchLogGroupName == "" {
		return nil, fmt.Errorf("logs not configured, please specify a \"CloudWatchLogGroupName\"")
	}

	prefix := fmt.Sprintf("%s_%s", buildTaskDefinitionTag(c.name, namespace, podName), containerName)
	describeResult, err := client.logsapi.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(c.cloudWatchLogGroupName),
		LogStreamNamePrefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	// Nothing logged yet.
	if len(describeResult.LogStreams) == 0 {
		return nil, nil
	}

	logs := ""

	err = client.logsapi.GetLogEventsPages(&cloudwatchlogs.GetLogEventsInput{
		Limit:         aws.Int64(int64(opts.Tail)),
		LogGroupName:  aws.String(c.cloudWatchLogGroupName),
		LogStreamName: describeResult.LogStreams[0].LogStreamName,
	}, func(page *cloudwatchlogs.GetLogEventsOutput, lastPage bool) bool {
		for _, event := range page.Events {
			logs += *event.Message
			logs += "\n"
		}

		// Due to a issue in the aws-sdk last page is never true, but the we can stop
		// as soon as no further results are returned.
		// See https://github.com/aws/aws-sdk-ruby/pull/730.
		return len(page.Events) > 0
	})

	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(strings.NewReader(logs)), nil
}
