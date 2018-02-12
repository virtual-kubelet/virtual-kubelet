package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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

// NewAcsCredential returns an AcsCredential struct from file path
func NewAcsCredential(filepath string) (*AcsCredential, error) {
	log.Printf("Reading ACS credential file %q", filepath)

	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("Reading ACS credential file %q failed: %v", filepath, err)
	}

	// Unmarshal the authentication file.
	var cred AcsCredential
	if err := json.Unmarshal(b, &cred); err != nil {
		return nil, err
	}

	log.Printf("Load ACS credential file %q successfully", filepath)
	return &cred, nil
}