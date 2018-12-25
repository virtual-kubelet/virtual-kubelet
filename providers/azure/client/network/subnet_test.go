package network

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-08-01/network"
)

func TestCreateGetSubnet(t *testing.T) {
	c := newTestClient(t)

	subnet := NewSubnetWithContainerInstanceDelegation(t.Name(), "10.0.0.0/24")

	ensureVnet(t, t.Name())

	s1, err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet)
	if err != nil {
		t.Fatal(err)
	}
	if s1 == nil {
		t.Fatal("create subnet should return subnet")
	}
	if s1.ID == nil || *s1.ID == "" {
		t.Fatal("create subnet should return subnet.ID")
	}

	var s2 *network.Subnet
	s2, err = c.GetSubnet(resourceGroup, t.Name(), *subnet.Name)
	if err != nil {
		t.Fatal(err)
	}
	if *s2.Name != *subnet.Name {
		t.Fatal("got unexpected subnet")
	}
	if *s2.SubnetPropertiesFormat.AddressPrefix != *subnet.SubnetPropertiesFormat.AddressPrefix {
		t.Fatalf("got unexpected address prefix: %s", *s2.SubnetPropertiesFormat.AddressPrefix)
	}
	if len(*s2.SubnetPropertiesFormat.Delegations) != 1 {
		t.Fatalf("got unexpected delgations: %v", *s2.SubnetPropertiesFormat.Delegations)
	}
	if *(*s2.SubnetPropertiesFormat.Delegations)[0].Name != *(*subnet.SubnetPropertiesFormat.Delegations)[0].Name {
		t.Fatalf("got unexpected delegation: %v", (*s2.SubnetPropertiesFormat.Delegations)[0])
	}
}
