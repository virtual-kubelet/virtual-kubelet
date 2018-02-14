package azure

import (
	"io/ioutil"
	"os"
	"testing"
)

const cred = `
{
    "cloud":"AzurePublicCloud",
    "tenantId": "72f988bf-86f1-41af-91ab-2d7cd011db47",
    "subscriptionId": "11122233-4444-5555-6666-777888999000",
    "aadClientId": "123",
    "aadClientSecret": "456",
    "resourceGroup": "vk-test-rg",
    "location": "westcentralus",
    "subnetName": "k8s-subnet",
    "securityGroupName": "k8s-master-nsg",
    "vnetName": "k8s-vnet",
    "routeTableName": "k8s-master-routetable",
    "primaryAvailabilitySetName": "agentpool1-availabilitySet",
    "cloudProviderBackoff": true,
    "cloudProviderBackoffRetries": 6,
    "cloudProviderBackoffExponent": 1.5,
    "cloudProviderBackoffDuration": 5,
    "cloudProviderBackoffJitter": 1,
    "cloudProviderRatelimit": true,
    "cloudProviderRateLimitQPS": 3,
    "cloudProviderRateLimitBucket": 10
}`

func TestAcsCred(t *testing.T) {
	file, err := ioutil.TempFile("", "acs_test")
	if err != nil {
		t.Error(err)
	}

	defer os.Remove(file.Name())

	if _, err := file.Write([]byte(cred)); err != nil {
		t.Error(err)
	}

	cred, err := NewAcsCredential(file.Name())
	if err != nil {
		t.Error(err)
	}
	wanted := "AzurePublicCloud"
	if cred.Cloud != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.Cloud)
	}

	wanted = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	if cred.TenantID != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.TenantID)
	}

	wanted = "11122233-4444-5555-6666-777888999000"
	if cred.SubscriptionID != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.SubscriptionID)
	}
	wanted = "123"
	if cred.ClientID != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.ClientID)
	}

	wanted = "456"
	if cred.ClientSecret != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.ClientSecret)
	}

	wanted = "vk-test-rg"
	if cred.ResourceGroup != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.ResourceGroup)
	}

	wanted = "westcentralus"
	if cred.Region != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, cred.Region)
	}
}

func TestAcsCredFileNotFound(t *testing.T) {
	file, err := ioutil.TempFile("", "acs_test")
	if err != nil {
		t.Error(err)
	}

	fileName := file.Name()

	if err := file.Close(); err != nil {
		t.Error(err)
	}

	os.Remove(fileName)

	if _, err := NewAcsCredential(fileName); err == nil {
		t.Fatal("expected to fail with bad json")
	}
}

const credBad = `
{
    "cloud":"AzurePublicCloud",
    "tenantId": "72f988bf-86f1-41af-91ab-2d7cd011db47",
    "subscriptionId": "11122233-4444-5555-6666-777888999000",
    "aadClientId": "123",
    "aadClientSecret": "456",
    "resourceGroup": "vk-test-rg",
    "location": "westcentralus",
	"subnetName": "k8s-subnet",`

func TestAcsCredBadJson(t *testing.T) {
	file, err := ioutil.TempFile("", "acs_test")
	if err != nil {
		t.Error(err)
	}

	defer os.Remove(file.Name())

	if _, err := file.Write([]byte(credBad)); err != nil {
		t.Error(err)
	}

	if _, err := NewAcsCredential(file.Name()); err == nil {
		t.Fatal("expected to fail with bad json")
	}
}
