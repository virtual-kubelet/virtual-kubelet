require '/mts/git/nimbus/lib/testframeworks/testng/testng.rb'
oneGB = 1 * 1000 * 1000
$testbed = Proc.new do |sharedStorageStyle, esxStyle|
  esxStyle ||= 'fullInstall'
  esxStyle = esxStyle.to_s
  sharedStorageStyle = sharedStorageStyle.to_s

  testbed = {
    'version' => 3,
    'name' => "nfs-1esx-#{sharedStorageStyle}-#{esxStyle}",
    'esx' => [
      {
        'name' => "esx.0",
        'style' => esxStyle,
        'numMem' => 16 * 1024,
        'numCPUs' => 6,
        "disks" => [ 30 * oneGB, 30 * oneGB, 30 * oneGB],
        'disableNfsMounts' => true,
        'nics' => 2,
        'staf' => false,
        'desiredPassword' => 'e2eFunctionalTest',
    }],
    'nfs' => [{
    'name' => 'dev-nfs',
    'type' => 'NFS',
    'nfsOpt' => 'rw',
    'numMem' => 16 * 1024,
    },
    'nfs' => {
    'name' => 'dev-nfs',
    'type' => 'NFS',
    'nfsOpt' => 'ro',
    'numMem' => 16 * 1024,
    }],
  }
end

[:pxeBoot, :fullInstall].each do |esxStyle|
    [:iscsi, :fc].each do |sharedStorageStyle|
      testbedSpec = $testbed.call(sharedStorageStyle, esxStyle)
      Nimbus::TestbedRegistry.registerTestbed testbedSpec
    end
  end
