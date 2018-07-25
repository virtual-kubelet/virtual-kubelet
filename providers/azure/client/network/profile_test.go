package network

import (
	"path"
	"testing"
)

func TestCreateGetProfile(t *testing.T) {
	c := newTestClient(t)
	ensureVnet(t, t.Name())

	subnet := &Subnet{
		Name: t.Name(),
		Properties: &SubnetProperties{
			AddressPrefix: "10.0.0.0/24",
		},
	}
	if err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet); err != nil {
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

	if err := c.CreateOrUpdateProfile(resourceGroup, p); err != nil {
		t.Fatal(err)
	}

	p2, err := c.GetProfile(resourceGroup, p.Name)
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
