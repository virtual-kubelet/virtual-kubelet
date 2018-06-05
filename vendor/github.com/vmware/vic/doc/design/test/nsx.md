## NSX Initial testing notes

##Required HW setup:
- vSphere 6.0/6.5 Cluster setup (6.5 wan't supported for a while, but the latest releases supports it)
- An ESX host in the cluster with a minimum of 4 CPU to host the NSX Manager Appliance

##Initial install:

###Deploy a Nimbus Cluster using 5-2-Cluster test.

###Add a beefy ESXi to the cluster to host the NSX appliance.

- `nimbus-esxdeploy --disk=40000000 --nics=2 --memory=90000 --cpus=4 nsx-esx 3620759`

###Update host password for the ESXi:

- `export GOVC_URL=root:@10.x.x.x` 

- `govc host.account.update -id root -password xxxxxx` 

###Add host to the VC Cluster:

- `export GOVC_URL="Administrator@vSphere.local":password@10.x.x.x`

- `govc cluster.add -hostname=10.x.x.x -username=root -dc=ha-datacenter -password=xxxx -noverify=true`

###Install the NSX manager using OVFTool.

- `ovftool nsx-manager-1.1.0.0.0.4788147.ova nsx-manager-1.1.0.0.0.4788147.ovf`

- `ovftool --datastore=${datastore} --name=${name} --net:"Network 1"="${network}" --diskMode=thin --powerOn --X:waitForIp --X:injectOvfEnv --X:enableHiddenProperties --prop:vami.domain.NSX=mgmt.local --prop:vami.searchpath.NSX=mgmt.local --prop:vami.DNS.NSX=8.8.8.8 --prop:vm.vmname=NSX nsx-manager-1.1.0.0.0.4788147.ovf 'vi://${user}:${password}@${host}`

###Add ESX nodes into the NSX Manager using the NSX REST API. 

- `FABRIC->Nodes using ESXi credentials: root/password`

###Create the Transport Zone using the NSX REST api

###Create a logical Switch with the VLAN based Transport Zone

###Add the Logical Switch as a Transport node to the ESXi host

###Check if the switch is visible from running govc command (Upgrade to the latest govc 0.12.0 to get this working)
- `govc ls network`

###Install VIC Appliance and Run Regression tests

