# Please make sure https://www.powershellgallery.com/packages/PSWindowsUpdate/2.1.0.1 is installed.
if (Get-Command -Module PSWindowsUpdate -errorAction SilentlyContinue)
{
    Write-Output "Running windows update"
    Install-WindowsUpdate -AcceptAll -IgnoreReboot
} 
else
{
    Write-Error "PSWindowsUpdate is not installed, please check https://www.powershellgallery.com/packages/PSWindowsUpdate/"
}
