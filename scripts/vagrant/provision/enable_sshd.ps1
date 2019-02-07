# Taken from https://docs.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse
# We need to make sure the latest updates are installed as mentioned in https://github.com/MicrosoftDocs/windowsserverdocs/issues/2074

Write-Output "Enabling OpenSSH"

Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

# Set services to start automatically on boot.
Set-Service sshd -StartupType Automatic

# Start the services for the first time.
Start-Service sshd
