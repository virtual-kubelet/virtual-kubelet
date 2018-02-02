package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
)

const (
	// AcsCredentialFilepathName defines the name of the environment variable
	// containing the path to the file which contains the ACS credential
	AcsCredentialFilepathName = "ACS_CREDENTIAL_LOCATION"
)

// AcsCredential represents the credential file for ACS
type AcsCredential struct {
	Cloud          string `json:"cloud"`
	TenantID       string `json:"tenantId"`
	SubscriptionID string `json:"subscriptionId"`
	ClientID       string `json:"aadClientId"`
	ClientSecret   string `json:"aadClientSecret"`
	ResourceGroup  string `json:"resourceGroup"`
	Region         string `json:"location"`
}

// NewAcsCredential returns an AcsCredential struct from file located
// at ACS_CREDENTIAL.
func NewAcsCredential() (*AcsCredential, error) {
	file := os.Getenv(AcsCredentialFilepathName)
	if file == "" {
		return nil, nil
	}

	log.Printf("Reading ACS credential file %q", file)

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Reading ACS credential file %q failed: %v", file, err)
	}

	// Unmarshal the authentication file.
	var cred AcsCredential
	if err := json.Unmarshal(b, &cred); err != nil {
		return nil, err
	}

	log.Printf("Load ACS credential file %q successfully", file)
	return &cred, nil
}

// ToAzureAuth converts the ACS credential to Azure Authentication class
func (cred *AcsCredential) ToAzureAuth() (*azure.Authentication, error) {
	if cred == nil {
		return nil, nil
	}

	switch cred.Cloud {
	case azure.PublicCloud.Name:
		return &azure.Authentication{
			ClientID:                   cred.ClientID,
			ClientSecret:               cred.ClientSecret,
			SubscriptionID:             cred.SubscriptionID,
			TenantID:                   cred.TenantID,
			ActiveDirectoryEndpoint:    azure.PublicCloud.ActiveDirectoryEndpoint,
			ResourceManagerEndpoint:    azure.PublicCloud.ResourceManagerEndpoint,
			GraphResourceID:            azure.PublicCloud.GraphEndpoint,
			SQLManagementEndpoint:      azure.PublicCloud.SQLDatabaseDNSSuffix,
			GalleryEndpoint:            azure.PublicCloud.GalleryEndpoint,
			ManagementEndpoint:         azure.PublicCloud.ServiceManagementEndpoint,
		}, nil
	default:
		return nil, fmt.Errorf("ACI only supports Public Azure. '%v' is not supported", cred.Cloud)
	}
}