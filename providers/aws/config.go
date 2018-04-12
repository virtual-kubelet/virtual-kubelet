package aws

import (
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
)

type providerConfig struct {
	Region             string
	Cluster            string
	Subnets            []string
	SecurityGroups     []string
	ExecutionRoleArn   string
	CloudWatchLogGroup string
	CPU                string
	Memory             string
	Pods               string
}

func (p *Provider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}

	if config.Region == "" {
		return fmt.Errorf("Region is required")
	}
	p.region = aws.String(config.Region)

	if config.Cluster == "" {
		return fmt.Errorf("Cluster is required")
	}
	p.cluster = aws.String(config.Cluster)

	if config.CloudWatchLogGroup == "" {
		return fmt.Errorf("CloudWatchLogGroup is required")
	}
	p.cloudWatchLogGroup = aws.String(config.CloudWatchLogGroup)

	if config.ExecutionRoleArn == "" {
		return fmt.Errorf("ExecutionRoleArn is required")
	}
	p.executionRoleArn = aws.String(config.ExecutionRoleArn)

	if len(config.Subnets) == 0 {
		return fmt.Errorf("At least one subnet is required")
	}
	p.subnets = toAWSStrings(config.Subnets)

	p.securityGroups = toAWSStrings(config.SecurityGroups)

	p.cpu = "20"
	if config.CPU != "" {
		p.cpu = config.CPU
	}

	p.memory = "100Gi"
	if config.Memory != "" {
		p.memory = config.Memory
	}

	p.pods = "20"
	if config.Pods != "" {
		p.pods = config.Pods
	}

	return nil
}

func toAWSStrings(items []string) []*string {
	awsStrings := make([]*string, len(items))

	for i, item := range items {
		awsStrings[i] = aws.String(item)
	}

	return awsStrings
}

func toStrings(items []*string) []string {
	stringList := make([]string, len(items))

	for i, item := range items {
		stringList[i] = *item
	}

	return stringList
}
