package azure

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"unicode/utf16"

	"github.com/dimchansky/utfbom"
)

// Authentication represents the authentication file for Azure.
type Authentication struct {
	ClientID                string `json:"clientId,omitempty"`
	ClientSecret            string `json:"clientSecret,omitempty"`
	SubscriptionID          string `json:"subscriptionId,omitempty"`
	TenantID                string `json:"tenantId,omitempty"`
	ActiveDirectoryEndpoint string `json:"activeDirectoryEndpointUrl,omitempty"`
	ResourceManagerEndpoint string `json:"resourceManagerEndpointUrl,omitempty"`
	GraphResourceID         string `json:"activeDirectoryGraphResourceId,omitempty"`
	SQLManagementEndpoint   string `json:"sqlManagementEndpointUrl,omitempty"`
	GalleryEndpoint         string `json:"galleryEndpointUrl,omitempty"`
	ManagementEndpoint      string `json:"managementEndpointUrl,omitempty"`
}

// NewAuthentication returns an authentication struct from user provided
// credentials.
func NewAuthentication(azureCloud, clientID, clientSecret, subscriptionID, tenantID string) *Authentication {
	environment := PublicCloud

	switch azureCloud {
	case PublicCloud.Name:
		environment = PublicCloud
		break
	case USGovernmentCloud.Name:
		environment = USGovernmentCloud
		break
	case ChinaCloud.Name:
		environment = ChinaCloud
		break
	case GermanCloud.Name:
		environment = GermanCloud
		break
	}

	return &Authentication{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		SubscriptionID:          subscriptionID,
		TenantID:                tenantID,
		ActiveDirectoryEndpoint: environment.ActiveDirectoryEndpoint,
		ResourceManagerEndpoint: environment.ResourceManagerEndpoint,
		GraphResourceID:         environment.GraphEndpoint,
		SQLManagementEndpoint:   environment.SQLDatabaseDNSSuffix,
		GalleryEndpoint:         environment.GalleryEndpoint,
		ManagementEndpoint:      environment.ServiceManagementEndpoint,
	}
}

// NewAuthenticationFromFile returns an authentication struct from file path
func NewAuthenticationFromFile(filepath string) (*Authentication, error) {
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("Reading authentication file %q failed: %v", filepath, err)
	}

	// Authentication file might be encoded.
	decoded, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("Decoding authentication file %q failed: %v", filepath, err)
	}

	// Unmarshal the authentication file.
	var auth Authentication
	if err := json.Unmarshal(decoded, &auth); err != nil {
		return nil, err
	}
	return &auth, nil

}

func decode(b []byte) ([]byte, error) {
	reader, enc := utfbom.Skip(bytes.NewReader(b))

	switch enc {
	case utfbom.UTF16LittleEndian:
		u16 := make([]uint16, (len(b)/2)-1)
		err := binary.Read(reader, binary.LittleEndian, &u16)
		if err != nil {
			return nil, err
		}
		return []byte(string(utf16.Decode(u16))), nil
	case utfbom.UTF16BigEndian:
		u16 := make([]uint16, (len(b)/2)-1)
		err := binary.Read(reader, binary.BigEndian, &u16)
		if err != nil {
			return nil, err
		}
		return []byte(string(utf16.Decode(u16))), nil
	}
	return ioutil.ReadAll(reader)
}
