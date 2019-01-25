# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure('2') do |config|
  config.vm.define 'win10' do |cfg|
    cfg.vm.box = 'StefanScherer/windows_10'
    cfg.vm.communicator = 'winrm'

    cfg.vm.synced_folder '.', 'C:\Go\src\gitlab.com\gitlab-org\gitlab-runner'

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/base.ps1'
  end

  config.vm.provider 'virtualbox' do |vb|
    vb.gui = false
    vb.memory = '2048'
    vb.cpus = 1
    vb.linked_clone = true
  end
end
