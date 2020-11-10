# -*- mode: ruby -*-
# vi: set ft=ruby :

# Check if the required plugins are installed.
unless Vagrant.has_plugin?('vagrant-reload')
  puts 'vagrant-reload plugin not found, installing'
  system 'vagrant plugin install vagrant-reload'
  # Restart the process with the plugin installed.
  exec "vagrant #{ARGV.join(' ')}"
end

def get_vm_box_version()
  # We're pinning to this specific version due to recent Docker versions (above 19.03.05) being broken
  # (see https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27115)
  '2020.04.15'
end

Vagrant.configure('2') do |config|
  config.vm.define 'windows_server', primary: true do |cfg|
    cfg.vm.box = 'StefanScherer/windows_2019_docker'
    cfg.vm.box_version = get_vm_box_version()
    cfg.vm.communicator = 'winrm'

    cfg.vm.synced_folder '.', 'C:\GitLab-Runner'

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/base.ps1'
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/install_PSWindowsUpdate.ps1'
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/windows_update.ps1'

    # Restart the box to install the updates, and update again.
    cfg.vm.provision :reload
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/windows_update.ps1'
    cfg.vm.provision :reload

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/enable_sshd.ps1'
  end

  config.vm.define 'windows_10', autostart: false do |cfg|
    cfg.vm.box = 'StefanScherer/windows_10'
    cfg.vm.box_version = get_vm_box_version()
    cfg.vm.communicator = 'winrm'

    cfg.vm.synced_folder '.', 'C:\GitLab-Runner'

    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/base.ps1'
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/enable_developer_mode.ps1'
    cfg.vm.provision 'shell', path: 'scripts/vagrant/provision/enable_sshd.ps1'
  end

  config.vm.provider 'virtualbox' do |vb|
    vb.gui = false
    vb.memory = '2048'
    vb.cpus = 1
    vb.linked_clone = true
  end
end
