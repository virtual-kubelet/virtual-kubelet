package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// AcsCredential represents the credential file for ACS
type AcsCredential struct {
	Cloud             string `json:"cloud"`
	TenantID          string `json:"tenantId"`
	SubscriptionID    string `json:"subscriptionId"`
	ClientID          string `json:"aadClientId"`
	ClientSecret      string `json:"aadClientSecret"`
	ResourceGroup     string `json:"resourceGroup"`
	Region            string `json:"location"`
	VNetName          string `json:"vnetName"`
	VNetResourceGroup string `json:"vnetResourceGroup"`
}

// NewAcsCredential returns an AcsCredential struct from file path
func NewAcsCredential(p string) (*AcsCredential, error) {
	logger := log.G(context.TODO()).WithField("method", "NewAcsCredential").WithField("file", p)
	logger.Debug("Reading ACS credential file")

	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("Reading ACS credential file %q failed: %v", p, err)
	}

	// Unmarshal the authentication file.
	var cred AcsCredential
	if err := json.Unmarshal(b, &cred); err != nil {
		return nil, err
	}

	logger.Debug("Load ACS credential file successfully")
	return &cred, nil
}
