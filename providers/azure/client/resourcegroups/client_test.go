package resourcegroups

import (
	"testing"

	"github.com/google/uuid"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
)

var (
	client        *Client
	location      = "eastus"
	resourceGroup = "virtual-kubelet-tests"
)

func init() {
	// Create a resource group name with uuid.
	uid := uuid.New()
	resourceGroup += "-" + uid.String()[0:6]
}

func TestNewClient(t *testing.T) {
	auth, err := azure.NewAuthenticationFromFile("../../../../credentials.json")
	if err != nil {
		t.Fatalf("Failed to load Azure authentication file: %v", err)
	}

	c, err := NewClient(auth)
	if err != nil {
		t.Fatal(err)
	}

	client = c
}

func TestResourceGroupDoesNotExist(t *testing.T) {
	exists, err := client.ResourceGroupExists(resourceGroup)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("resource group should not exist before it has been created")
	}
}

func TestCreateResourceGroup(t *testing.T) {
	g, err := client.CreateResourceGroup(resourceGroup, Group{
		Location: location,
	})
	if err != nil {
		t.Fatal(err)
	}
	// check the name is the same
	if g.Name != resourceGroup {
		t.Fatalf("resource group name is %s, expected virtual-kubelet-tests", g.Name)
	}
}

func TestResourceGroupExists(t *testing.T) {
	exists, err := client.ResourceGroupExists(resourceGroup)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("resource group should exist after being created")
	}
}

func TestGetResourceGroup(t *testing.T) {
	g, err := client.GetResourceGroup(resourceGroup)
	if err != nil {
		t.Fatal(err)
	}
	// check the name is the same
	if g.Name != resourceGroup {
		t.Fatalf("resource group name is %s, expected %s", g.Name, resourceGroup)
	}
}

func TestDeleteResourceGroup(t *testing.T) {
	err := client.DeleteResourceGroup(resourceGroup)
	if err != nil {
		t.Fatal(err)
	}
}
