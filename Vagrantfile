# -*- mode: ruby -*-
# vi: set ft=ruby :

# Check if the required plugins are installed.
unless Vagrant.has_plugin?('vagrant-reload')
  puts 'vagrant-reload plugin not found, installing'
  system 'vagrant plugin install vagrant-reload'
  # Restart the process with the plugin installed.
  exec "vagrant #{ARGV.join(' ')}"
end

Vagrant.configure('2') do |config|
  config.vm.define 'win10' do |cfg|
    cfg.vm.box = 'StefanScherer/windows_2019_docker'
    cfg.vm.communicator = 'winrm'

    cfg.vm.synced_folder '.', 'C:\Go\src\gitlab.com\gitlab-org\gitlab-runner'

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/base.ps1'
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/windows_update.ps1'

    # Restart the box to install the updates, and update again.
    cfg.vm.provision :reload
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/windows_update.ps1'
    cfg.vm.provision :reload

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/enable_sshd.ps1'
  end

  config.vm.provider 'virtualbox' do |vb|
    vb.gui = false
    vb.memory = '2048'
    vb.cpus = 1
    vb.linked_clone = true
  end
end
