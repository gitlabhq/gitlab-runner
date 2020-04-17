$goVersion = "1.13.8"
$gitVersion = "2.23.0"
$powerShellCoreVersion = "7.0.0"
$srcFolder = "C:\GitLab-Runner"

[environment]::SetEnvironmentVariable("RUNNER_SRC", $srcFolder, "Machine")

Write-Host "Installing Chocolatey"
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

Write-Host "Installing Go"
choco install golang -y --version $goVersion

Write-Host "Installing Git"
choco install git -y --version $gitVersion

Write-Host "Install PowerShell Core"
choco install powershell-core -y --version $powerShellCoreVersion
