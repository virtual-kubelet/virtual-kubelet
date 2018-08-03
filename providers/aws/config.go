package aws

import (
	"fmt"
	"io"
	"os"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/aws/fargate"

	"github.com/BurntSushi/toml"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// Provider configuration defaults.
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/platform_versions.html
	defaultPlatformVersion         = "LATEST"
	defaultClusterName             = "default"
	defaultAssignPublicIPv4Address = false
	defaultOperatingSystem         = providers.OperatingSystemLinux

	// Default resource capacity advertised by Fargate provider.
	// These are intentionally low to prevent any accidental overuse.
	defaultCPUCapacity     = "20"
	defaultMemoryCapacity  = "40Gi"
	defaultStorageCapacity = "40Gi"
	defaultPodCapacity     = "20"

	// Minimum resource capacity advertised by Fargate provider.
	// These values correspond to the minimum Fargate task size.
	minCPUCapacity    = "250m"
	minMemoryCapacity = "512Mi"
	minPodCapacity    = "1"
)

// ProviderConfig represents the contents of the provider configuration file.
type providerConfig struct {
	Region                  string
	ClusterName             string
	Subnets                 []string
	SecurityGroups          []string
	AssignPublicIPv4Address bool
	ExecutionRoleArn        string
	CloudWatchLogGroupName  string
	PlatformVersion         string
	OperatingSystem         string
	CPU                     string
	Memory                  string
	Storage                 string
	Pods                    string
}

// loadConfigFile loads the given Fargate provider configuration file.
func (p *FargateProvider) loadConfigFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	err = p.loadConfig(f)
	return err
}

// loadConfig loads the given Fargate provider TOML configuration stream.
func (p *FargateProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	var q resource.Quantity

	// Set defaults for optional fields.
	config.ClusterName = defaultClusterName
	config.AssignPublicIPv4Address = defaultAssignPublicIPv4Address
	config.PlatformVersion = defaultPlatformVersion
	config.OperatingSystem = defaultOperatingSystem
	config.CPU = defaultCPUCapacity
	config.Memory = defaultMemoryCapacity
	config.Storage = defaultStorageCapacity
	config.Pods = defaultPodCapacity

	// Read the user-supplied configuration.
	_, err := toml.DecodeReader(r, &config)
	if err != nil {
		return err
	}

	// Validate aggregate configuration.
	if config.Region == "" {
		return fmt.Errorf("Region is a required field")
	}
	if !fargate.FargateRegions.Include(config.Region) {
		return fmt.Errorf(
			"Fargate is available only in regions %v and not available in %v",
			fargate.FargateRegions.Names(), config.Region)
	}
	if config.Subnets == nil || len(config.Subnets) == 0 {
		return fmt.Errorf("Subnets is a required field")
	}
	if config.SecurityGroups == nil {
		config.SecurityGroups = []string{}
	}
	if config.OperatingSystem != providers.OperatingSystemLinux {
		return fmt.Errorf("Fargate does not support operating system %v", config.OperatingSystem)
	}
	if config.CloudWatchLogGroupName != "" && config.ExecutionRoleArn == "" {
		return fmt.Errorf("Execution role required if CloudWatch log group is specified")
	}

	// Validate advertised capacity.
	if q, err = resource.ParseQuantity(config.CPU); err != nil {
		return fmt.Errorf("Invalid CPU value %v", config.CPU)
	}
	if q.Cmp(resource.MustParse(minCPUCapacity)) == -1 {
		return fmt.Errorf("CPU value %v is less than the minimum %v", config.CPU, minCPUCapacity)
	}
	if q, err = resource.ParseQuantity(config.Memory); err != nil {
		return fmt.Errorf("Invalid memory value %v", config.Memory)
	}
	if q.Cmp(resource.MustParse(minMemoryCapacity)) == -1 {
		return fmt.Errorf("Memory value %v is less than the minimum %v", config.Memory, minMemoryCapacity)
	}
	if q, err = resource.ParseQuantity(config.Storage); err != nil {
		return fmt.Errorf("Invalid storage value %v", config.Storage)
	}
	if q, err = resource.ParseQuantity(config.Pods); err != nil {
		return fmt.Errorf("Invalid pods value %v", config.Pods)
	}
	if q.Cmp(resource.MustParse(minPodCapacity)) == -1 {
		return fmt.Errorf("Pod value %v is less than the minimum %v", config.Pods, minPodCapacity)
	}

	// Populate provider fields.
	p.region = config.Region
	p.subnets = config.Subnets
	p.securityGroups = config.SecurityGroups

	p.clusterName = config.ClusterName
	p.assignPublicIPv4Address = config.AssignPublicIPv4Address
	p.executionRoleArn = config.ExecutionRoleArn
	p.cloudWatchLogGroupName = config.CloudWatchLogGroupName
	p.platformVersion = config.PlatformVersion
	p.operatingSystem = config.OperatingSystem
	p.capacity.cpu = config.CPU
	p.capacity.memory = config.Memory
	p.capacity.storage = config.Storage
	p.capacity.pods = config.Pods

	return nil
}
