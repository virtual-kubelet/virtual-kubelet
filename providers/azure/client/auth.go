package azure

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"unicode/utf16"

	"github.com/dimchansky/utfbom"
)

const (
	// AuthenticationFilepathName defines the name of the environment variable
	// containing the path to the file to be used to populate Authentication
	// for Azure.
	AuthenticationFilepathName = "AZURE_AUTH_LOCATION"
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
func NewAuthentication(clientID, clientSecret, subscriptionID, tenantID string) *Authentication {
	return &Authentication{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		SubscriptionID: subscriptionID,
		TenantID:       tenantID,
	}
}

// NewAuthenticationFromFile returns an authentication struct from file located
// at AZURE_AUTH_LOCATION.
func NewAuthenticationFromFile() (*Authentication, error) {
	file := os.Getenv(AuthenticationFilepathName)
	if file == "" {
		return nil, fmt.Errorf("Authentication file not found, environment variable %s is not set", AuthenticationFilepathName)
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Reading authentication file %q failed: %v", file, err)
	}

	// Authentication file might be encoded.
	decoded, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("Decoding authentication file %q failed: %v", file, err)
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
