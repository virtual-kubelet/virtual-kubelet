Test 6-07 - Verify vic-machine create network function
=======

# Purpose:
Verify vic-machine create public, management, bridge network and container networks functions

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available



# Public network

## Public network - default
1. Create without public network provided
2. Verify "VM Network" is connected in VCH VM
3. Integration test passed

## Public network - invalid
1. Create with wrong network name provided for public network
2. Verify create failed for network is not found

## Public network - invalid vCenter
1. Create with distribute virtual switch as public network name
2. Verify create failed for network type is wrong

## Public network - DHCP
1. Create with network name no DHCP availabile for public network
2. Verify VCH created but without ip address
3. Verify VCH can be deleted without anything wrong through vic-machine delete

## Public network - valid
1. Create with DPG as public network in VC and correct switch in ESXi
2. Verify create passed
3. Verify integration test passed

# Management network

## Management network - none
1. Create without management network provided, but public network correctly set
2. Verify warning message set for management network and client network sharing the same network
3. No multiple attachement in VM network to same vSphere virtual switch (or DPG)
4. Integration test passed

## Management network - invalid
1. Create with wrong network name provided for management network
2. Verify create failed for network is not found

## Management network - invalid vCenter
1. Create with distribute virtual switch as management network name
2. Verify create failed for network type is wrong

## Management network - unreachable
1. Create with network unreachable for vSphere or VC as management network
2. Verify VCH created but VC or vSphere is unreachable
3. Make sure vic-machine failed with user-friendly error message

## Management network - valid
1. Create with correct management network (switch for ESX, DPG for vCenter)
2. Verify create passed
3. Verify integration test passed



# Bridge network

## Bridge network - vCenter none
1. Create without bridge network provided in VC
2. Create failed for bridge network should be specified in VC

## Bridge network - ESX none
1. Create without bridge network provided in ESXi
2. Integration test pass

## Bridge network - create bridge network if it doesn't exist
1. Create with wrong network name provided for bridge network
2. Verify create failed for network is not found, create will succeed on ESXi

## Bridge network - invalid vCenter
3. Create with distribute virtual switch as bridge network name
4. Verify create failed for network type is wrong

## Bridge network - non-DPG
1. Create with standard network in VC as bridge network
2. vic-machine failed for DPG is required for bridge network

## Bridge network - valid
1. Create with DPG in VC and switch in ESXi
2. Verify create passed
3. Verify integration test passed

## Bridge network - reused port group
1. Create with same network for bridge and public network
2. Verify create failed for same network with public network
3. Same case with management network
4. Same case with container network

## Bridge network - invalid IP settings
1. Create with bridge network correctly set
2. Set bridge network IP range with wrong format
3. Verify create failed with user-friendly error message

## Bridge network - invalid bridge network range
1. Create with bridge network IP range smaller than /16
2. Verify create failed with user-friendly error message

## Bridge network - valid with IP range
1. Create with bridge network correctly set
2. Set bridge network ip range correctly
3. Verify create passed
4. Regression test passed
5. docker create container, with ip address correctly set in the above ip range


# Container network

## Container network - space in network name invalid
1. Create with container network <Net With Spaces> and <Net With Spaces>: and <Net With Spaces>:<Alias>s
2. Verify create failed with a network name must be supplied for <Net With Space>

## Container network - space in network name valid
1. Create with container network: <Net With Spaces>:vmnet
2. Verify create passed
3. Regression test passed
4. Verify docker network ls command to show net1 network

## Container network invalid 1
1. Create with invalid container network: <WrongNet>:alias
2. Verify create failed with WrongNet is not found

## Container network invalid 2
1. Create with container network: <standard switch network name>:alias in VC
2. Verify create failed with standard network is not supported

## Container network 1
1. Create with container network: <dpg name>:net1 in VC or <standard switch network name>:net1 in ESXi
2. Verify create passed
3. Regression test passed
4. Verify docker network ls command to show net1 network

## Container network 2
1. Create with container network: <dpg name> in VC or <standard switch network name> in ESXi
2. Verify create passed
3. Regression test passed
4. Verify docker network ls command to show the <vsphere network name> network

## Network mapping invalid
1. Create with two container network map to same alias
2. Verify create failed with two different vsphere network map to same docker network

## Network mapping gateway invalid
1. Create with container network mapping
2. Set container network gateway as <dpg name>:1.1.1.1/24
3. Set container network gateway as <dpg name>:192.168.1.0/24
4. Set container network gateway as <wrong name>:192.168.1.0/24
5. Verify create failed for wrong vsphere network name or gateway is not routable

## Network mapping IP invalid
1. Create with container network mapping
2. Set container ip range as <wrong name>:192.168.2.1-192.168.2.100
3. Set container network gateway as <dpg name>:192.168.1.1/24, and container ip range as <dpg name>:192.168.2.1-192.168.2.100
4. Verify create failed for wrong vsphere network name or ip range is wrong

## DNS format invalid
1. Create with container network mapping
2. Set container DNS as <wrong name>:8.8.8.8
3. Set container DNS as <dpg name>:abcdefg
4. Verify create failed for wrong vsphere name or wrong dns format

## Network mapping
1. Create with container network mapping <dpg name>:net1
2. Set container network gateway as <dpg name>:192.168.1.1/24
3. Set container ip range as <dpg name>:192.168.1.2-192.168.1.100
4. Set container DNS as <dpg name>:<correct dns>
5. Verify create passed
6. Integration test passed
7. Docker network ls show net1
8. Docker container created with network attached with net1, got ip address inside of network range
9. Docker create another container, and link to previous one, can talk to the the first container successfully

## Container Firewalls
1. Create an open container and verify another open container can connect to it on arbitrary ports.
2. a. Try to publish a port on a closed firewall and verify an error is received.
   b. Create a closed container and verify an open container cannot connect to it on an arbitrary port.
   c. Create a container connected to a bridge and a closed network. Verify that another container connected
      to the same bridge can connect to the closed container.
3. a. Create an outbound container. Verify that an outbound container on the same external network cannot 
      connect to the first outbound container.
   b. Verify that the outbound container can initiate a connection with an open network on an arbitrary port.
   c. Verify that two outbound containers on the same external network and on the same bridge network can
      talk to one another via hte bridge.
4. a. Create a published container that publishes port 1337. Verify that an outbound container can connect to port
      1337 on the published container.
   b. Verify that an outbound container cannot connect to any other arbitrary port on the published container.
5. a. Create a peer network `A` with ip range `10.10.10.0/24` and gateway `10.10.10.1/24`.
   b. Create a peer network `B` with ip range `192.168.0.0/16` and gateway `192.168.0.1/16`.
   c. Verify that a container on network `B` cannot connect to network `A` through an arbitrary port.
   d. Verify that a new container on network `A` (a peer) can connect to another container on network `A`
      on an arbitrary port.
6. Verify that a closed container can ping localhost

# VCH static IP

## VCH static IP - Static public
1. Create with static IP address for public network (client and management networks unspecified
   default to same port group as public network)
2. Verify debug output shows specified static IP address correctly assigned and copied to client and
   management networks

## VCH static IP - Static client
1. Create with static IP address for client network and specify client, public, and management
   networks to be on same port group
2. Verify debug output shows specified static IP address correctly assigned and copied to public
   and management networks

## VCH static IP - Static management
1. Create with static IP address for management network and specify client, public, and management
   networks to be on the same port group
2. Verify debug output shows specified static IP address correctly assigned and copied to client
   and public management networks

## VCH static IP - different port groups 1
1. Create with static IP address for public network and specify client and management networks to
   be on different port group
2. Verify debug output shows specified static IP address correctly assigned
3. Verify debug output shows client and management networks set to DHCP

## VCH static IP - different port groups 2
1. Create with static IP address for public network on `public-network` port group and a static
   IP address for client network on `client-network` port group
2. Verify debug output shows correct IP address assigned to each interface

## VCH static IP - same port group
1. Create with static IP address for each public network and client network and specify both to be
   on the same port group
2. Verify output shows configuration error and install does not proceed

## VCH static IP - same subnet for multiple port groups
1. Create with static IP address for public network and a static IP address for client network.
   Specify the addresses to be on the same subnet, but assign each network to a different port
   group
2. Verify output shows warning that assigning the same subnet to different port groups is
   unsupported
