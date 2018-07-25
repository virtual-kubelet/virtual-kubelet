package aci

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/resourcegroups"
)

var (
	client         *Client
	location       = "eastus"
	resourceGroup  = "virtual-kubelet-tests"
	containerGroup = "virtual-kubelet-test-container-group"
)

func init() {
	// Create a resource group name with uuid.
	uid := uuid.New()
	resourceGroup += "-" + uid.String()[0:6]
}

// The TestMain function creates a resource group for testing
// and deletes in when it's done.
func TestMain(m *testing.M) {
	auth, err := azure.NewAuthenticationFromFile("../../../../credentials.json")
	if err != nil {
		log.Fatalf("Failed to load Azure authentication file: %v", err)
	}

	// Check if the resource group exists and create it if not.
	rgCli, err := resourcegroups.NewClient(auth)
	if err != nil {
		log.Fatalf("creating new resourcegroups client failed: %v", err)
	}

	// Check if the resource group exists.
	exists, err := rgCli.ResourceGroupExists(resourceGroup)
	if err != nil {
		log.Fatalf("checking if resource group exists failed: %v", err)
	}

	if !exists {
		// Create the resource group.
		_, err := rgCli.CreateResourceGroup(resourceGroup, resourcegroups.Group{
			Location: location,
		})
		if err != nil {
			log.Fatalf("creating resource group failed: %v", err)
		}
	}

	// Run the tests.
	merr := m.Run()

	// Delete the resource group.
	if err := rgCli.DeleteResourceGroup(resourceGroup); err != nil {
		log.Printf("Couldn't delete resource group %q: %v", resourceGroup, err)

	}

	if merr != 0 {
		os.Exit(merr)
	}

	os.Exit(0)
}

func TestNewClient(t *testing.T) {
	auth, err := azure.NewAuthenticationFromFile("../../../../credentials.json")
	if err != nil {
		log.Fatalf("Failed to load Azure authentication file: %v", err)
	}

	c, err := NewClient(auth)
	if err != nil {
		t.Fatal(err)
	}

	client = c
}

func TestCreateContainerGroupFails(t *testing.T) {
	_, err := client.CreateContainerGroup(resourceGroup, containerGroup, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected create container group to fail with ResourceRequestsNotSpecified, but returned nil")
	}

	if !strings.Contains(err.Error(), "ResourceRequestsNotSpecified") {
		t.Fatalf("expected ResourceRequestsNotSpecified to be in the error message but got: %v", err)
	}
}

func TestCreateContainerGroupWithoutResourceLimit(t *testing.T) {
	cg, err := client.CreateContainerGroup(resourceGroup, containerGroup, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != containerGroup {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, containerGroup)
	}

	if err := client.DeleteContainerGroup(resourceGroup, containerGroup); err != nil {
		t.Fatal(err)
	}
}

func TestCreateContainerGroup(t *testing.T) {
	cg, err := client.CreateContainerGroup(resourceGroup, containerGroup, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != containerGroup {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, containerGroup)
	}
}

func TestGetContainerGroup(t *testing.T) {
	cg, err, _ := client.GetContainerGroup(resourceGroup, containerGroup)
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != containerGroup {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, containerGroup)
	}
}

func TestListContainerGroup(t *testing.T) {
	list, err := client.ListContainerGroups(resourceGroup)
	if err != nil {
		t.Fatal(err)
	}
	for _, cg := range list.Value {
		if cg.Name != containerGroup {
			t.Fatalf("resource group name is %s, expected %s", cg.Name, containerGroup)
		}
	}
}

func TestCreateContainerGroupWithLivenessProbe(t *testing.T) {
	uid := uuid.New()
	congainerGroupName := containerGroup + "-" + uid.String()[0:6]
	cg, err := client.CreateContainerGroup(resourceGroup, congainerGroupName, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
						LivenessProbe: &ContainerProbe{
							HTTPGet: &ContainerHTTPGetProbe{
								Port: 80,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != congainerGroupName {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, congainerGroupName)
	}
}

func TestCreateContainerGroupFailsWithLivenessProbeMissingPort(t *testing.T) {
	uid := uuid.New()
	congainerGroupName := containerGroup + "-" + uid.String()[0:6]
	_, err := client.CreateContainerGroup(resourceGroup, congainerGroupName, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
						LivenessProbe: &ContainerProbe{
							HTTPGet: &ContainerHTTPGetProbe{
								Path: "/",
							},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
}

func TestCreateContainerGroupWithReadinessProbe(t *testing.T) {
	uid := uuid.New()
	congainerGroupName := containerGroup + "-" + uid.String()[0:6]
	cg, err := client.CreateContainerGroup(resourceGroup, congainerGroupName, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
						ReadinessProbe: &ContainerProbe{
							HTTPGet: &ContainerHTTPGetProbe{
								Port: 80,
								Path: "/",
							},
							InitialDelaySeconds: 5,
							SuccessThreshold:    3,
							FailureThreshold:    5,
							TimeoutSeconds:      120,
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != congainerGroupName {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, congainerGroupName)
	}
}

func logAnalyticsWorkspaceFromFile(filepath string) (*LogAnalyticsWorkspace, error) {
	analyticsdata, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("Reading LogAnalyticsWorkspace file %q failed: %v", filepath, err)
	}
	// Unmarshal the log analytics file.
	var law LogAnalyticsWorkspace
	if err := json.Unmarshal(analyticsdata, &law); err != nil {
		return nil, err
	}
	return &law, nil
}

func TestCreateContainerGroupWithLogAnalytics(t *testing.T) {
	law, err := logAnalyticsWorkspaceFromFile("../../../../loganalytics.json")
	if err != nil {
		t.Fatal(err)
	}
	cgname := "cgla"
	cg, err := client.CreateContainerGroup(resourceGroup, cgname, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
					},
				},
			},
			Diagnostics: &ContainerGroupDiagnostics{
				LogAnalytics: law,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cg.Name != cgname {
		t.Fatalf("resource group name is %s, expected %s", cg.Name, cgname)
	}
	if err := client.DeleteContainerGroup(resourceGroup, cgname); err != nil {
		t.Fatalf("Delete Container Group failed: %s", err.Error())
	}
}

func TestCreateContainerGroupWithInvalidLogAnalytics(t *testing.T) {
	law := &LogAnalyticsWorkspace{}
	_, err := client.CreateContainerGroup(resourceGroup, containerGroup, ContainerGroup{
		Location: location,
		ContainerGroupProperties: ContainerGroupProperties{
			OsType: Linux,
			Containers: []Container{
				{
					Name: "nginx",
					ContainerProperties: ContainerProperties{
						Image:   "nginx",
						Command: []string{"nginx", "-g", "daemon off;"},
						Ports: []ContainerPort{
							{
								Protocol: ContainerNetworkProtocolTCP,
								Port:     80,
							},
						},
						Resources: ResourceRequirements{
							Requests: &ResourceRequests{
								CPU:        1,
								MemoryInGB: 1,
							},
							Limits: &ResourceLimits{
								CPU:        1,
								MemoryInGB: 1,
							},
						},
					},
				},
			},
			Diagnostics: &ContainerGroupDiagnostics{
				LogAnalytics: law,
			},
		},
	})
	if err == nil {
		t.Fatal("TestCreateContainerGroupWithInvalidLogAnalytics should fail but encountered no errors")
	}
}

func TestDeleteContainerGroup(t *testing.T) {
	err := client.DeleteContainerGroup(resourceGroup, containerGroup)
	if err != nil {
		t.Fatal(err)
	}
}
