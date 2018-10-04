package network

import (
	"path"
	"testing"
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

	subnet := &Subnet{
		Name: t.Name(),
		Properties: &SubnetProperties{
			AddressPrefix: "10.0.0.0/24",
		},
	}

	subnet, err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet)
	if err != nil {
		t.Fatal(err)
	}

	p := &Profile{
		Name:     t.Name(),
		Type:     "Microsoft.Network/networkProfiles",
		Location: location,
		Properties: ProfileProperties{
			ContainerNetworkInterfaceConfigurations: []InterfaceConfiguration{
				{
					Name: "eth0",
					Properties: InterfaceConfigurationProperties{
						IPConfigurations: []IPConfiguration{
							{
								Name: "ipconfigprofile1",
								Properties: IPConfigurationProperties{
									Subnet: ID{
										ID: subnet.ID,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	p1, err := c.CreateOrUpdateProfile(resourceGroup, p)
	if err != nil {
		t.Fatal(err)
	}
	if p1 == nil {
		t.Fatal("create profile should return profile")
	}
	if p1.ID == "" {
		t.Fatal("create profile should return profile.ID")
	}

	var p2 *Profile
	p2, err = c.GetProfile(resourceGroup, p.Name)
	if err != nil {
		t.Fatal(err)
	}

	if len(p2.Properties.ContainerNetworkInterfaceConfigurations) != 1 {
		t.Fatalf("got unexpected profile properties: %+v", p2.Properties)
	}
	if len(p2.Properties.ContainerNetworkInterfaceConfigurations[0].Properties.IPConfigurations) != 1 {
		t.Fatalf("got unexpected profile IP configuration: %+v", p2.Properties.ContainerNetworkInterfaceConfigurations[0].Properties.IPConfigurations)
	}
	if p2.Properties.ContainerNetworkInterfaceConfigurations[0].Properties.IPConfigurations[0].Properties.Subnet.ID != subnet.ID {
		t.Fatal("got unexpected subnet")
	}

	subnet, err = c.GetSubnet(resourceGroup, t.Name(), t.Name())
	if err != nil {
		t.Fatal(err)
	}

	if len(subnet.Properties.IPConfigurationProfiles) != 1 {
		t.Fatalf("got unexpected subnet IP configuration profiles: %+v", subnet.Properties.IPConfigurationProfiles)
	}

	expected := path.Join(p2.ID, "containerNetworkInterfaceConfigurations/eth0/ipConfigurations/ipconfigprofile1")
	if subnet.Properties.IPConfigurationProfiles[0].ID != expected {
		t.Fatalf("got unexpected profile, expected:\n\t%s, got:\n\t%s", expected, subnet.Properties.IPConfigurationProfiles[0].ID)
	}
}
