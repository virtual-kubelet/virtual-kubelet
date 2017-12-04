package resourcegroups

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
)

var (
	client        *Client
	location      = "eastus"
	resourceGroup = "virtual-kubelet-tests"
)

func init() {
	// Check if the AZURE_AUTH_LOCATION variable is already set.
	// If it is not set, set it to the root of this project in a credentials.json file.
	if os.Getenv("AZURE_AUTH_LOCATION") == "" {
		// Check if the credentials.json file exists in the root of this project.
		_, filename, _, _ := runtime.Caller(0)
		dir := filepath.Dir(filename)
		file := filepath.Join(dir, "../../../../credentials.json")

		// Check if the file exists.
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Fatalf("Either set AZURE_AUTH_LOCATION or add a credentials.json file to the root of this project.")
		}

		// Set the environment variable for the authentication file.
		os.Setenv("AZURE_AUTH_LOCATION", file)
	}

	// Create a resource group name with uuid.
	uid := uuid.New()
	resourceGroup += "-" + uid.String()[0:6]
}

func TestNewClient(t *testing.T) {
	var err error
	client, err = NewClient()
	if err != nil {
		t.Fatal(err)
	}
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
