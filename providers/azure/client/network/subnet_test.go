package network

import "testing"

func TestCreateGetSubnet(t *testing.T) {
	c := newTestClient(t)

	subnet := &Subnet{
		Name: t.Name(),
		Properties: &SubnetProperties{
			AddressPrefix: "10.0.0.0/24",
			Delegations: []Delegation{
				{Name: "aciDelegation", Properties: DelegationProperties{
					ServiceName: "Microsoft.ContainerInstance/containerGroups",
					Actions:     []string{"Microsoft.Network/virtualNetworks/subnets/action"},
				}},
			},
		},
	}
	ensureVnet(t, t.Name())

	if err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet); err != nil {
		t.Fatal(err)
	}

	s, err := c.GetSubnet(resourceGroup, t.Name(), subnet.Name)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != subnet.Name {
		t.Fatal("got unexpected subnet")
	}
	if s.Properties.AddressPrefix != subnet.Properties.AddressPrefix {
		t.Fatalf("got unexpected address prefix: %s", s.Properties.AddressPrefix)
	}
	if len(s.Properties.Delegations) != 1 {
		t.Fatalf("got unexpected delgations: %v", s.Properties.Delegations)
	}
	if s.Properties.Delegations[0].Name != subnet.Properties.Delegations[0].Name {
		t.Fatalf("got unexpected delegation: %v", s.Properties.Delegations[0])
	}
}
