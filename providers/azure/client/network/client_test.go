package network

import (
	"context"
	"sync"
	"testing"

	"github.com/Azure/go-autorest/autorest"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-08-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

var vnetAuthOnce sync.Once
var azAuth autorest.Authorizer

func ensureVnet(t *testing.T, name string) {
	vnetAuthOnce.Do(func() {
		var err error
		azAuth, err = auth.NewClientCredentialsConfig(testAuth.ClientID, testAuth.ClientSecret, testAuth.TenantID).Authorizer()
		if err != nil {
			t.Fatalf("error setting up client auth for vnet create: %v", err)
		}
	})

	client := network.NewVirtualNetworksClient(testAuth.SubscriptionID)
	client.Authorizer = azAuth

	prefixes := []string{"10.0.0.0/24"}
	result, err := client.CreateOrUpdate(context.Background(), resourceGroup, name, network.VirtualNetwork{
		Name:     &name,
		Location: &location,
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &prefixes,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := result.WaitForCompletion(context.Background(), client.Client); err != nil {
		t.Fatal(err)
	}
}
