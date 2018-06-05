Test 5-7 - NSX
=======

# Purpose:
To verify the VIC appliance works when the vCenter is using NSX-v networking

# References:
[1 - VMware NSX](http://www.vmware.com/products/nsx.html)

# Environment:
This test requires access to VMWare Nimbus cluster for dynamic ESXi and vCenter creation

# Test Steps:
1. Deploy NSX
	`/mts/git/bin/nimbus-vsmdeploy --nics 1 --vsmBuild ob-5007049 vic-3-nsxv-mgr`

2. Deploy VC + VSAN
	`/mts/git/bin/nimbus-testbeddeploy --noSupportBundles --vcvaBuild 4944578 --esxPxeDir 4887370 --esxBuild 4887370 --testbedName vcqa-vsan- simple-pxeBoot-vcva --runName vsan`

3. Register NSX-v to VC from Manage vCenter Registration, to add the VC info (NSX Credential: admin/default).

4. Assign NSXv License in VC from Assets → Solutions → All Actions → Assign License → Add License → Select the new added license → OK

5. Create a Distributed Switch
   - Edit the new added DVS MTU value of DVS from default 1500 to 9000
   - Select the new added DVS and Add all hosts in the cluster
   - Assign vmnic1 for this switch per host (remain vmnic0 with vSwitch0 to get the external IP)

6. Prepare IP of NSX Controller
   - Select one host which will create NSX controller in.
   - Login the host via ssh, get host's network info 1). Netmask  2). Gateway.
     - `esxcli network ip interface ipv4 get`

7. Install a NSX Controller from Networking & Security
   - Add Name, select dc, cluster, datastore as usual
   - Modify to VM Network rather than Distributed Port Group. Because Controller need to get an external IP
   - Go to IP Pool Select New IP Pool
   - Define Gateway, Prefix Length got from #VC Preparation section
   - In Static IP Pool part, add <unused-IP>-<unused-IP>. For example: 10.162.57.43-10.162.57.43
   - Click ok to create the controller.

8. Install NSX components on ESX from Home, Networking & Security, Installation, Host Preparation to all the host in cluster.

9. Configure VXLAN
   - Click Not Configured in VXLAN column → Keep value as default except 'VMKNic IP Addressing'
     `Default MTU is 1600, make sure the MTU of DVS is larger than this one (set as 9000 manually previously)`
   - For 'VMKNic IP Addressing' field, default is 'Use DHCP', change it to 'Use IP Pool'.
   - Choose 'New IP Pool', add new ip pool info.
      - `It's recommended to define an internal IP pool in order to isolate from management network`
      - `Example: Name → internel-vxlan-ip-pool, Gateway → 192.168.0.254, Prefix Legth → 24`
      - `Static IP Pool → 192.168.0.1-192.168.0.10 (Depends on how many host this VXLAN managed, length of IP pool > num of hosts)`
   - Check VXLAN is installed on ESX (vmk1 is newly created)
   - Show the vmknic info of VXLAN from Web Client: ( Installation → Logical Network Preparation → VXLAN Transport)
   - Check the network settings from the host
      - `esxcli network ip interface list`
      - `net-vdl2 -l`
   - Check vmknic created by VXLAN among all hosts are ping-able
      - `esxcli network ip interface ipv4 get`

10. Create Logical Switch
    - Go to Home → Networking & Security → Installation → Logical Network Preparation
    - Remain info in 'VXLAN Transport' as default
    - Go to 'Segment ID' → Edit → Add the value of 'Segment ID pool', for example '5001-50000'
    - Go to 'Transport Zones' → Click Add → Create a Transport Zone → Add Name / Description properly,
       - `Remain Replication mode as 'Unicast', Select   cluster that will be part of this Transport Zones`

11. Deploy VCH Appliance to the new vCenter

12. Run a variety of docker commands on the VCH appliance

# Expected Outcome:
The VCH appliance should deploy without error and each of the docker commands executed against it should return without error

# Possible Problems:
None
