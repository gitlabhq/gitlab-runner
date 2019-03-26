# Install https://www.powershellgallery.com/packages/PSWindowsUpdate so tha we
# can manually download windows update.
Write-Output "Installing PSWindowsUpdate module"

# Make sure we can download from the Powershell Gallery https://www.powershellgallery.com/
Install-PackageProvider -Name NuGet -Force
Set-PSRepository -Name PSGallery -InstallationPolicy Trusted

# Install the actual module.
Install-Module -Name PSWindowsUpdate -Force
