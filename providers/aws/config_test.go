package aws

import (
	"bytes"
	"log"
	"testing"
)

const cfg = `
Region = "us-east-1"
Cluster = "test-cluster-123"
CloudWatchLogGroup = "test-group"
ExecutionRoleArn = "role"
Subnets = [ "subnet-a", "subnet-b", "subnet-c" ]
SecurityGroups = [ "sg-a" ]
`

func TestConfig(t *testing.T) {
	br := bytes.NewReader([]byte(cfg))
	var p Provider
	err := p.loadConfig(br)
	if err != nil {
		t.Fatal(err)
	}
	wanted := "us-east-1"
	if *p.region != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, *p.region)
	}

	log.Printf("%v", p.subnets)

	wanted = "subnet-a"
	if *p.subnets[0] != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, *p.subnets[0])
	}

	wanted = "sg-a"
	if *p.securityGroups[0] != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, *p.securityGroups[0])
	}
}
