# -*- mode: ruby -*-

Vagrant.configure(2) do |config|
  dirs = ENV['GOPATH'] || Dir.home
  gdir = nil
  config.ssh.forward_agent = true
  config.vm.define "vic_dev" do | vic_dev |
    vic_dev.vm.box = 'bento/ubuntu-16.04'
    vic_dev.vm.network 'forwarded_port', guest: 2375, host: 12375, auto_correct: true
    vic_dev.vm.host_name = 'devbox'
    vic_dev.vm.synced_folder '.', '/vagrant', disabled: true
    vic_dev.ssh.username = 'vagrant'

    dirs.split(File::PATH_SEPARATOR).each do |dir|
      gdir = dir.sub("C\:", "/C")
      vic_dev.vm.synced_folder dir, gdir
    end

    vic_dev.vm.provider :virtualbox do |v, _override|
      v.memory = 4096
      v.cpus = 2
    end

    [:vmware_fusion, :vmware_workstation].each do |visor|
      vic_dev.vm.provider visor do |v, _override|
        v.memory = 4096
        v.cpus = 2

        v.vmx["ethernet0.pcislotnumber"] = "32"
      end
    end

    Dir['infra/machines/devbox/provision.sh', 'infra/machines/devbox/provision-drone.sh'].each do |path|
      vic_dev.vm.provision 'shell', path: path, args: [gdir, vic_dev.ssh.username]
    end
  end
end
