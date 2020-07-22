Set-StrictMode -Version latest
$ErrorActionPreference = "Stop"
$InformationPreference = "Continue"
# ---------------------------------------------------------------------------
# This script depends on a few environment variables that should be populated
# before running the script:
#
# - $Env:WINDOWS_VERSION - This is the version of windows that is going to be
#   used for building the Docker image. It is important for the version to match
#   one of the mcr.microsoft.com/windows/servercore or https://hub.docker.com/_/microsoft-windows-nanoserver
#   tag prefixes (discarding the architecture suffix).
#   For example, `servercore1903` will build from mcr.microsoft.com/windows/servercore:1903-amd64.
# - $Env:GIT_VERSION - Specify which version of Git needs to be installed on
#   the Docker image. This is done through Docker build args.
# - $Env:GIT_VERSION_BUILD - Specify which build is needed to download for the
#   GIT_VERSION you specified.
# - $Env:GIT_256_CHECKSUM - The checksum of the downloaded zip, usually found in
#   the GitHub release page.
# - $Env:GIT_LFS_VERSION - The Git LFS version needed to install on the
#   Docker image.
# - $Env:GIT_LFS_256_CHECKSUM - The checksum of the downloaded zip, usually
#   found in the GitHub release page.
# - $Env:IS_LATEST - When we want to tag current tag as the latest, this is usually
#   used when we are tagging a release for the runner (which is not a patch
#   release or RC)
# - $Env:DOCKER_HUB_USER - The user we want to login with for docker hub.
# - $Env:DOCKER_HUB_PASSWORD - The password we want to login with for docker hub.
# - $Env:PUSH_TO_DOCKER_HUB - If set to true, it will login to the registry and
#   push the tags.
# ---------------------------------------------------------------------------
$imagesBasePath = "dockerfiles\runner-helper\Dockerfile.x86_64"

function Main
{
    $tag = Get-Tag

    Build-Image $tag

    if (-not ($Env:PUSH_TO_DOCKER_HUB -eq "true"))
    {
        '$Env:PUSH_TO_DOCKER_HUB is not true, done'
        return
    }

    Connect-Registry

    Push-Tag $tag

    if ($Env:IS_LATEST -eq "true")
    {
        Add-LatestTag $tag
        Push-Latest
    }

    Disconnect-Registry
}

function Get-Tag
{
    $revision = & 'git' rev-parse --short=8 HEAD

    return "x86_64-$revision-$Env:WINDOWS_VERSION"
}

function Build-Image($tag)
{
    $windowsFlavor = $env:WINDOWS_VERSION.Substring(0, $env:WINDOWS_VERSION.length -4)
    $windowsVersion = $env:WINDOWS_VERSION.Substring($env:WINDOWS_VERSION.length -4)

    Write-Information "Build image for x86_64_${env:WINDOWS_VERSION}"

    $dockerFile = "${imagesBasePath}_${windowsFlavor}"
    $context = "dockerfiles\runner-helper"
    New-Item -ItemType Directory -Force -Path $context\binaries
    Copy-Item -Path "out\binaries\gitlab-runner-helper\gitlab-runner-helper.x86_64-windows.exe" -Destination "$context\binaries"
    $buildArgs = @(
        '--build-arg', "BASE_IMAGE_TAG=mcr.microsoft.com/windows/${windowsFlavor}:${windowsVersion}-amd64",
        '--build-arg', "GIT_VERSION=$Env:GIT_VERSION",
        '--build-arg', "GIT_VERSION_BUILD=$Env:GIT_VERSION_BUILD",
        '--build-arg', "GIT_256_CHECKSUM=$Env:GIT_256_CHECKSUM"
        '--build-arg', "GIT_LFS_VERSION=$Env:GIT_LFS_VERSION"
        '--build-arg', "GIT_LFS_256_CHECKSUM=$Env:GIT_LFS_256_CHECKSUM"
    )

    & 'docker' build -t "gitlab/gitlab-runner-helper:$tag" --force-rm --no-cache $buildArgs -f $dockerFile $context
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to build docker image" )
    }
}

function Push-Tag($tag)
{
    Write-Information "Push $tag"

    & 'docker' push gitlab/gitlab-runner-helper:$tag
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to push docker image gitlab/gitlab-runner-helper:$tag" )
    }
}

function Add-LatestTag($tag)
{
    Write-Information "Tag $tag as latest"

    & 'docker' tag "gitlab/gitlab-runner-helper:$tag" "gitlab/gitlab-runner-helper:x86_64-latest-$Env:WINDOWS_VERSION"
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to tag gitlab/gitlab-runner-helper:$tag as latest image" )
    }
}

function Push-Latest()
{
    Write-Information "Push latest tag"

    & 'docker' push "gitlab/gitlab-runner-helper:x86_64-latest-$Env:WINDOWS_VERSION"
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to push image to registry" )
    }
}

function Connect-Registry
{
    Write-Information 'Login docker hub'

    & 'docker' login --username $Env:DOCKER_HUB_USER --password $Env:DOCKER_HUB_PASSWORD
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to login Docker hub" )
    }
}

function Disconnect-Registry
{
    Write-Information 'Logout register'

    & 'docker' logout
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to logout from Docker hub" )
    }
}

Try
{
    if (-not (Test-Path env:WINDOWS_VERSION))
    {
        throw '$Env:WINDOWS_VERSION is not set'
    }

    Main
} 
Finally 
{
    if (-not (Test-Path env:SKIP_CLEANUP))
    {
        Write-Information "Cleaning up the build image"
        $tag = Get-Tag

        # We don't really care if these fail or not, clean up shouldn't fail
        # the pipelines.
        & 'docker' rmi -f gitlab/gitlab-runner-helper:$tag
        & 'docker' image prune -f
    }
}
