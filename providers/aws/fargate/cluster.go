package fargate

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// ClusterConfig contains a Fargate cluster's configurable parameters.
type ClusterConfig struct {
	Region                  string
	Name                    string
	NodeName                string
	Subnets                 []string
	SecurityGroups          []string
	AssignPublicIPv4Address bool
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
	platformVersion         string
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
		platformVersion:         config.PlatformVersion,
	}

	// Check if the cluster already exists.
	err = cluster.describe()
	if err != nil {
		return nil, err
	}

	// If not, try to create it.
	// This might fail if the role doesn't have the necessary permission.
	if cluster.arn == "" {
		err = cluster.create()
		if err != nil {
			return nil, err
		}
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

	c.arn = *output.Cluster.ClusterArn
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
	if err != nil || len(output.Clusters) > 1 {
		err = fmt.Errorf("failed to describe cluster: %v", err)
		log.Println(err)
		return err
	}

	if len(output.Clusters) == 0 {
		log.Printf("Fargate cluster %s in region %s does not exist.", c.name, c.region)
	} else {
		log.Printf("Found Fargate cluster %s in region %s.", c.name, c.region)
		c.arn = *output.Clusters[0].ClusterArn
	}

	return nil
}
