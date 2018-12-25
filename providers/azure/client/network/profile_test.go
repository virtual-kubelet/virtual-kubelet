package network

import (
	"path"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-08-01/network"
)

func TestGetProfileNotFound(t *testing.T) {
	c := newTestClient(t)
	p, err := c.GetProfile(resourceGroup, "someprofile")
	if err == nil {
		t.Fatalf("expect error when getting the non-exist profile: %v", p)
	}
	if !IsNotFound(err) {
		t.Fatal("expect NotFound error")
	}
	if p != nil {
		t.Fatal("unexpected profile")
	}
}

func TestCreateGetProfile(t *testing.T) {
	c := newTestClient(t)
	ensureVnet(t, t.Name())

	subnet := NewSubnetWithContainerInstanceDelegation(t.Name(), "10.0.0.0/24")

	subnet, err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet)
	if err != nil {
		t.Fatal(err)
	}

	p := NewNetworkProfile(t.Name(), location, *subnet.ID)

	p1, err := c.CreateOrUpdateProfile(resourceGroup, p)
	if err != nil {
		t.Fatal(err)
	}
	if p1 == nil {
		t.Fatal("create profile should return profile")
	}
	if p1.ID == nil || *p1.ID == "" {
		t.Fatal("create profile should return profile.ID")
	}

	var p2 *network.Profile
	p2, err = c.GetProfile(resourceGroup, *p.Name)
	if err != nil {
		t.Fatal(err)
	}

	if len(*p2.ProfilePropertiesFormat.ContainerNetworkInterfaceConfigurations) != 1 {
		t.Fatalf("got unexpected profile properties: %+v", *p2.ProfilePropertiesFormat)
	}

	containterNetworkInterfaceConfiguration := (*p2.ProfilePropertiesFormat.ContainerNetworkInterfaceConfigurations)[0]
	if len(*containterNetworkInterfaceConfiguration.ContainerNetworkInterfaceConfigurationPropertiesFormat.IPConfigurations) != 1 {
		t.Fatalf("got unexpected profile IP configuration: %+v", *containterNetworkInterfaceConfiguration.ContainerNetworkInterfaceConfigurationPropertiesFormat.IPConfigurations)
	}

	ipConfiguration := (*containterNetworkInterfaceConfiguration.ContainerNetworkInterfaceConfigurationPropertiesFormat.IPConfigurations)[0]
	if *ipConfiguration.IPConfigurationProfilePropertiesFormat.Subnet.ID != *subnet.ID {
		t.Fatal("got unexpected subnet")
	}

	subnet, err = c.GetSubnet(resourceGroup, t.Name(), t.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(*subnet.SubnetPropertiesFormat.IPConfigurationProfiles) != 1 {
		t.Fatalf("got unexpected subnet IP configuration profiles: %+v", *subnet.SubnetPropertiesFormat.IPConfigurationProfiles)
	}

	expected := path.Join(*p2.ID, "containerNetworkInterfaceConfigurations/eth0/ipConfigurations/ipconfigprofile1")
	if *(*subnet.SubnetPropertiesFormat.IPConfigurationProfiles)[0].ID != expected {
		t.Fatalf("got unexpected profile, expected:\n\t%s, got:\n\t%s", expected, *(*subnet.SubnetPropertiesFormat.IPConfigurationProfiles)[0].ID)
	}
}
