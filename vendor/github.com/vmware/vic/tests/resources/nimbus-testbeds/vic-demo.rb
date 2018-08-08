oneGB = 1 * 1000 * 1000 # in KB
 
$testbed = Proc.new do
  {
    "name" => "vic-iscsi-cluster",
    "version" => 3,
    "esx" => (0..2).map do | idx |
      {
        "name" => "esx.#{idx}",
        "vc" => "vc.0",
        "style" => "fullInstall",
        "desiredPassword" => "e2eFunctionalTest",
        "memory" => 24576, # 6x default
        "disks" => [ 30 * oneGB ],
        "nics" => 2,
	"cpus" => 6,
        "iScsi" => ["iscsi.0"],
        "clusterName" => "cls",
      }
    end,

    "iscsi" => [
      {
        "name" => "iscsi.0",
        "luns" => [512],
        "iqnRandom" => "nimbus1"
      }
    ],

    "vcs" => [
      {
        "name" => "vc.0",
        "type" => "vcva",
        "dcName" => "dc1",
        "clusters" => [{"name" => "cls", "vsan" => false, "enableDrs" => true, "enableHA" => true}],
        "addHosts" => "allInSameCluster",
      }
    ],

    "postBoot" => Proc.new do |runId, testbedSpec, vmList, catApi, logDir|
      esxList = vmList['esx']
        esxList.each do |host|
          host.ssh do |ssh|
            ssh.exec!("esxcli network firewall set -e false")
          end
      end
      vc = vmList['vc'][0]
      vim = VIM.connect vc.rbvmomiConnectSpec
      datacenters = vim.serviceInstance.content.rootFolder.childEntity.grep(RbVmomi::VIM::Datacenter)
      raise "Couldn't find a Datacenter precreated"  if datacenters.length == 0
      datacenter = datacenters.first
      Log.info "Found a datacenter successfully in the system, name: #{datacenter.name}"
      clusters = datacenter.hostFolder.children
      raise "Couldn't find a cluster precreated"  if clusters.length == 0
      cluster = clusters.first
      Log.info "Found a cluster successfully in the system, name: #{cluster.name}"

      dvs = datacenter.networkFolder.CreateDVS_Task(
        :spec => {
          :configSpec => {
            :name => "test-ds"
          },
	    }
      ).wait_for_completion
      Log.info "Vds DSwitch created"

      dvpg1 = dvs.AddDVPortgroup_Task(
        :spec => [
          {
            :name => "management",
            :type => :earlyBinding,
            :numPorts => 12,
          }
        ]
      ).wait_for_completion
      Log.info "management DPG created"

      dvpg2 = dvs.AddDVPortgroup_Task(
        :spec => [
          {
            :name => "vm-network",
            :type => :earlyBinding,
            :numPorts => 12,
          }
        ]
      ).wait_for_completion
      Log.info "vm-network DPG created"

      dvpg3 = dvs.AddDVPortgroup_Task(
        :spec => [
          {
            :name => "bridge",
            :type => :earlyBinding,
            :numPorts => 12,
          }
        ]
      ).wait_for_completion
      Log.info "bridge DPG created"

      Log.info "Add hosts to the DVS"
      onecluster_pnic_spec = [ VIM::DistributedVirtualSwitchHostMemberPnicSpec({:pnicDevice => 'vmnic1'}) ]
      dvs_config = VIM::DVSConfigSpec({
        :configVersion => dvs.config.configVersion,
        :host => cluster.host.map do |host|
        {
          :operation => :add,
          :host => host,
          :backing => VIM::DistributedVirtualSwitchHostMemberPnicBacking({
            :pnicSpec => onecluster_pnic_spec
          })
        }
        end
      })
      dvs.ReconfigureDvs_Task(:spec => dvs_config).wait_for_completion
      Log.info "Hosts added to DVS successfully"

      Log.info "Download and install latest VIC release"
      vc.ssh do |ssh|
        out = ssh.exec!("wget https://storage.googleapis.com/vic-engine-releases/vic_v1.4.0.tar.gz")
        Log.info out
        out = ssh.exec!("tar xzf vic_v1.4.0.tar.gz")
        Log.info out
        out = ssh.exec!("./vic/vic-machine-linux create -t #{vc.ip} -u #{vc.vimUsername} -p #{vc.vimPassword} --image-store sharedVmfs-0 --public-network vm-network --bridge-network bridge --compute-resource cls --no-tls --force")
        Log.info out
      end
    end
  }
end
