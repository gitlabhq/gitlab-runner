$goVersion = "1.8"
$gitVersion = "2.18.0"
$srcFolder = "C:\Go\src\gitlab.com\gitlab-org\gitlab-runner"

function Main
{
    [environment]::SetEnvironmentVariable("RUNNER_SRC", $srcFolder, "Machine")
    Install-Go($goVersion)
    Install-Git($gitVersion)
    Install-PSWindowsUpdate
}

function Install-Go([string]$version)
{
    $file = 'go' + $version +'.windows-amd64.msi'
    $url = 'https://storage.googleapis.com/golang/' + $file
    $dest = Download -Url $url


    Write-Host "installing go $version..."
    $logFile = Log-File -App 'go'
    $MSIArguments = @(
        "/i"
        ('"{0}"' -f $dest)
        "/qn"
        "/norestart"
        "/L*v"
        $logFile
    )
    Start-Process "msiexec.exe" -ArgumentList $MSIArguments -Wait -NoNewWindow

    Write-Host "done"
    Remove-Item $dest
}

function Install-Git([string]$version)
{
    $releaseVersion = 'v' + $version + '.windows.1'
    $file = 'Git-' + $version +'-64-bit.exe'
    $url = GitHubRelease -Project 'git-for-windows/git' -Version $releaseVersion -File $file
    $dest = Download -Url $url

    Write-Host "installing git $version..."
    $logFile = Log-File -App 'git'
    $InstallArguments = @(
        "/VERYSILENT"
        ('/LOG="{0}"' -f $logFile)
    )
    Start-Process $dest -ArgumentList $InstallArguments -Wait -NoNewWindow

    Write-Host "done"
    Remove-Item $dest
}

# Install https://www.powershellgallery.com/packages/PSWindowsUpdate so tha we
# can manually download windows update.
function Install-PSWindowsUpdate 
{
    Write-Output "Installing PSWindowsUpdate module"
    # Make sure we can download from the Powershell Gallery https://www.powershellgallery.com/
    Install-PackageProvider -Name NuGet -Force
    Set-PSRepository -Name PSGallery -InstallationPolicy Trusted

    # Install the actual module.
    Install-Module -Name PSWindowsUpdate -Force
}

function GitHubRelease([string]$Project, [string]$Version = 'latest', [string]$File) {
    'https://github.com/' + $Project + '/releases/download/' + $Version + '/' + $File
}

function Download([string]$Url) {
    $dest = [System.IO.Path]::GetTempFileName()
    [Net.ServicePointManager]::SecurityProtocol = "tls12, tls11"
    Write-Host "downloading $Url"

    # Create client, set its info, and download
    $wc = New-Object System.Net.WebClient
    $wc.UseDefaultCredentials = $true
    $wc.Headers.Add("X-FORMS_BASED_AUTH_ACCEPTED", "f")
    $wc.DownloadFile($Url, $dest)

    Write-Host "$url downloaded as $dest"
    $dest
}

function Log-File($App)
{
    $timestamp = get-date -Format yyyyMMddTHHmmss
    $logFile = '{0}-{1}.log' -f $App,$timestamp
    $vagrantFolder = Join-Path -Path $srcFolder -ChildPath ".vagrant"
    Join-Path -Path $vagrantFolder -ChildPath $logFile
}

Main
