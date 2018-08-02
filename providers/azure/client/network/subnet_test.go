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

	s1, err := c.CreateOrUpdateSubnet(resourceGroup, t.Name(), subnet)
	if err != nil {
		t.Fatal(err)
	}
	if s1 == nil {
		t.Fatal("create subnet should return subnet")
	}
	if s1.ID == "" {
		t.Fatal("create subnet should return subnet.ID")
	}

	var s2 *Subnet
	s2, err = c.GetSubnet(resourceGroup, t.Name(), subnet.Name)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Name != subnet.Name {
		t.Fatal("got unexpected subnet")
	}
	if s2.Properties.AddressPrefix != subnet.Properties.AddressPrefix {
		t.Fatalf("got unexpected address prefix: %s", s2.Properties.AddressPrefix)
	}
	if len(s2.Properties.Delegations) != 1 {
		t.Fatalf("got unexpected delgations: %v", s2.Properties.Delegations)
	}
	if s2.Properties.Delegations[0].Name != subnet.Properties.Delegations[0].Name {
		t.Fatalf("got unexpected delegation: %v", s2.Properties.Delegations[0])
	}
}
