oneGB = 1 * 1000 * 1000 # in KB
$testbed = Proc.new do |type, esxStyle, vcStyle, dbType, location|
  esxStyle ||= 'pxeBoot'
  vcStyle ||= 'vpxInstall'

  esxStyle = esxStyle.to_s
  vcStyle = vcStyle.to_s
  sharedStorageStyle = 'iscsi'
  dbType = dbType.to_s

  nameParts = ['vic', 'vsan', type, esxStyle, vcStyle]
  if dbType != 'embedded'
    nameParts << dbType
  end

  if type == 'complex'
    numEsx = 8
    numCPUs = 4
    vmsPerHost = 1
    addHosts = 'vsan-complex-vcqafvt'
  elsif type == 'simple'
    numEsx = 4
    numCPUs = 4
    vmsPerHost = 0
    addHosts = 'vsan-simple-vcqafvt'
  elsif type == 'complexCPU'
    numEsx = 8
    numCPUs = 2
    vmsPerHost = 1
    addHosts = 'vsan-complex-vcqafvt'
  else
    numEsx = 4
    numCPUs = 2
    vmsPerHost = 0
    addHosts = 'vsan-simple-vcqafvt'
  end
  testbed = {
    'name' => nameParts.join('-'),
    'esx' => (0...numEsx).map do |i|
      {
        'name' => "esx.#{i}",
        'style' => esxStyle,
        'disks' => [ 30 * oneGB, 30 * oneGB, 30 * oneGB],
        'freeLocalLuns' => 1,
        'freeSharedLuns' => 2,
        'numMem' => 13 * 1024,
        'numCPUs' => numCPUs,
        'ssds' => [ 5*oneGB ],
        'nics' => 2,
        'staf' => false,
        'desiredPassword' => 'e2eFunctionalTest',
        'vmotionNics' => ['vmk0'],
        'mountNfs' => ['nfs.0'],
      }
    end,
    'vc' => {
      'type' => vcStyle,
      'additionalScript' => [], # XXX: Create users
      'addHosts' => addHosts,
      'linuxCertFile' => TestngLauncher.vcvaCertFile,
      'dbType' => dbType,
    },
    'vsan' => true,
    'postBoot' => Proc.new do |runId, testbedSpec, vc, esxList, iscsiList|
      ovf = NimbusUtils.get_absolute_ovf("CentOS/VM_OVF10.ovf")
      if location && $nimbusEnv && $nimbusEnv['NIMBUS'] =~ /gamma/
        ovf = 'http://10.116.111.50/testwareTestCentOS/VM_OVF10.ovf'
      end

      cloudVc = vc
      vim = VIM.connect cloudVc.rbvmomiConnectSpec
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

      VcQaTestbedCommon.postBoot(
        vmsPerHost,
        NimbusUtils.get_absolute_ovf("CentOS/VM_OVF10.ovf"),
        runId, testbedSpec, vc, esxList, iscsiList)
    end,
    'nfs' => [
      {
        'name' => "nfs.0"
      },
    ],
  }

  if dbType == 'mssql'
    testbed['genericVM'] ||= []
    testbed['genericVM'] << {
      'name' => 'vc-mssql',
      'type' => 'mssql2k8',
    }
    testbed['vc']['dbHost'] = 'vc-mssql'
  end

  testbed = VcQaTestbedCommon.addSharedDisks testbed, [20, 10, 20, 10], sharedStorageStyle   # 2 x 20gb shared vmfs, 2 x 10gb free luns as defined by 'freeSharedLuns', DON'T CHANGE THE ORDERING UNLESS YOU KNOW WHAT YOU'RE DOING!

  testbed
end

[:pxeBoot, :fullInstall].each do |esxStyle|
  [:vpxInstall, :vcva].each do |vcStyle|
    [:embedded, :mssql, :oracle].each do |dbType|
      ['complex', 'simple', 'complexCPU', 'simpleCPU'].each do |type|
        testbedSpec = $testbed.call(type, esxStyle, vcStyle, dbType)
        Nimbus::TestbedRegistry.registerTestbed testbedSpec
      end
    end
  end
end
