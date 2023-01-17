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
#   For example, `servercoreYYH1` will build from mcr.microsoft.com/windows/servercore:YYH1-amd64.
# - $Env:GIT_VERSION - Specify which version of Git needs to be installed on
#   the Docker image. This is done through Docker build args.
# - $Env:GIT_VERSION_BUILD - Specify which build is needed to download for the
#   GIT_VERSION you specified.
# - $Env:GIT_WINDOWS_AMD64_CHECKSUM - The checksum of the downloaded zip, usually found in
#   the GitHub release page.
# - $Env:GIT_LFS_VERSION - The Git LFS version needed to install on the
#   Docker image.
# - $Env:GIT_LFS_WINDOWS_AMD64_CHECKSUM - The checksum of the downloaded .tar.gz file, usually
#   found in the GitHub release page.
# - $Env:PWSH_VERSION - The Powershell Core version needed to install on the
#   Docker image.
# - $Env:PWSH_WINDOWS_AMD64_CHECKSUM - The checksum of the downloaded MSI, usually
#   found in the GitHub release page.
# - $Env:IS_LATEST - When we want to tag current tag as the latest, this is usually
#   used when we are tagging a release for the runner (which is not a patch
#   release or RC)
# - $Env:DOCKER_HUB_USER - The user we want to login with for docker hub.
# - $Env:DOCKER_HUB_PASSWORD - The password we want to login with for docker hub.
# - $Env:PUSH_TO_DOCKER_HUB - If set to true, it will login to the registry and
#   push the tags.
# - $Env:DOCKER_HUB_NAMESPACE - Usually empty and only set for development, to
#   use your own namespace instead of `gitlab`.
# - $Env:SKIP_CLEANUP - By default this PowerShell script will delete the image
#   it just build.
# - $Env:CI_REGISTRY_IMAGE - Image name to push to GitLab registry. Usually set
#   by CI.
# - $Env:CI_REGISTRY - The GitLab registry name. Usually set by CI.
# - $Env:CI_REGISTRY_USER - The user used to login CI_REGISTRY. Usually set by
#   CI.
# - $Env:CI_REGISTRY_PASSWORD - The password used to login CI_REGISTRY. Usually
#   set by CI.
# - $Env:PUSH_TO_ECR_PUBLIC - If set to true, it will login to the registry and
#   push the tags.
# - $Env:ECR_PUBLIC_REGISTRY - The ecr public registry. If it's not defined it
#   will fallback to the default production registry.
# ---------------------------------------------------------------------------
$imagesBasePath = "dockerfiles/runner-helper/Dockerfile.x86_64"

function Main
{
    if (-not (Test-Path Env:IS_LATEST))
    {
        $Env:IS_LATEST = Is-Latest
    }

    $tag = Get-Tag

    Build-Image $tag

    if ($Env:PUBLISH_IMAGES -eq "true")
    {
        Connect-Registry $Env:CI_REGISTRY_USER $Env:CI_REGISTRY_PASSWORD $Env:CI_REGISTRY

        Push-Tag "${Env:CI_REGISTRY_IMAGE}" $tag
        Push-As-Ref "${Env:CI_REGISTRY_IMAGE}" $tag

        if ($Env:IS_LATEST -eq "true")
        {
            Push-As-Latest $Env:CI_REGISTRY_IMAGE $tag
        }

        Disconnect-Registry $env:CI_REGISTRY
    }

    if ($Env:PUSH_TO_DOCKER_HUB -eq "true")
    {
        $namespace = DockerHub-Namespace

        Connect-Registry $Env:DOCKER_HUB_USER $Env:DOCKER_HUB_PASSWORD
        Push-Tag $namespace $tag
        Push-As-Ref $namespace $tag

        if ($Env:IS_LATEST -eq "true")
        {
            Push-As-Latest $namespace $tag
        }

        Disconnect-Registry
    }

    if ($Env:PUSH_TO_ECR_PUBLIC -eq "true")
    {
        $ecrPublicRegistry = ECR-Public-Registry
        $ecrPublicRegistryPassword = & aws --region us-east-1 ecr-public get-login-password

        Connect-Registry AWS $ecrPublicRegistryPassword $ecrPublicRegistry

        Push-Tag $ecrPublicRegistry $tag
        Push-As-Ref $ecrPublicRegistry $tag

        if ($Env:IS_LATEST -eq "true")
        {
            Push-As-Latest $ecrPublicRegistry $tag
        }

        Disconnect-Registry $ecrPublicRegistry
    }
}

function Get-Tag
{
    $revision = & 'git' rev-parse --short=8 HEAD

    return "x86_64-$revision-$Env:WINDOWS_VERSION"
}

function Get-Latest-Stable-Tag
{
    $versions = & git -c versionsort.prereleaseSuffix="-rc" -c versionsort.prereleaseSuffix="-RC" tag -l "v*.*.*" |
        Where-Object { $_ -notlike "*-rc*" } |
        %{[System.Version]$_.Substring(1)} |
        sort -descending
    $latestTag = $versions[0].ToString()

    return "v$latestTag"
}

function Is-Latest
{
    $prevErrorPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue' # We expect errors from `git describe`, so temporarily disable handling

    try
    {
        $latestTag = Get-Latest-Stable-Tag
        & git describe --exact-match --match $latestTag 2>&1 | out-null
        $isLatest = $LASTEXITCODE -eq 0
    }
    finally
    {
        $ErrorActionPreference = $prevErrorPreference
    }

    return $isLatest
}

function Build-Image($tag)
{
    $windowsFlavor = $env:WINDOWS_VERSION.Substring(0, $env:WINDOWS_VERSION.length -4)
    $windowsVersion = $env:WINDOWS_VERSION.Substring($env:WINDOWS_VERSION.length -4)
    $dockerHubNamespace = DockerHub-Namespace
    $ecrPublicRegistry = ECR-Public-Registry

    if ($windowsVersion -eq "21H1") {
        $windowsVersion = "ltsc2022"
    }

    Write-Information "Build image for x86_64_${env:WINDOWS_VERSION}"

    $dockerFile = "${imagesBasePath}_${windowsFlavor}"
    $context = "dockerfiles/runner-helper"
    New-Item -ItemType Directory -Force -Path $context\binaries
    Copy-Item -Path "out\binaries\gitlab-runner-helper\gitlab-runner-helper.x86_64-windows.exe" -Destination "$context/binaries"
    $buildArgs = @(
        '--build-arg', "BASE_IMAGE_TAG=mcr.microsoft.com/windows/${windowsFlavor}:${windowsVersion}-amd64",
        '--build-arg', "PWSH_VERSION=$Env:PWSH_VERSION",
        '--build-arg', "PWSH_AMD64_CHECKSUM=$Env:PWSH_WINDOWS_AMD64_CHECKSUM",
        '--build-arg', "GIT_VERSION=$Env:GIT_VERSION",
        '--build-arg', "GIT_VERSION_BUILD=$Env:GIT_VERSION_BUILD",
        '--build-arg', "GIT_AMD64_CHECKSUM=$Env:GIT_WINDOWS_AMD64_CHECKSUM"
        '--build-arg', "GIT_LFS_VERSION=$Env:GIT_LFS_VERSION"
        '--build-arg', "GIT_LFS_AMD64_CHECKSUM=$Env:GIT_LFS_WINDOWS_AMD64_CHECKSUM"
    )

    $imageNames = @(
        '-t', "$Env:CI_REGISTRY_IMAGE/gitlab-runner-helper:$tag",
        '-t', "$dockerHubNamespace/gitlab-runner-helper:$tag",
        '-t', "$ecrPublicRegistry/gitlab-runner-helper:$tag"
    )

    & 'docker' build $imageNames --force-rm --no-cache $buildArgs -f $dockerFile $context
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to build docker image" )
    }
}

function Push-Tag($namespace, $tag)
{
    Write-Information "Push $tag"

    & 'docker' push ${namespace}/gitlab-runner-helper:$tag
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to push docker image ${namespace}/gitlab-runner-helper:$tag" )
    }
}

function Push-As-Latest($namespace, $tag)
{
    Push-As $namespace $tag "latest"
}

function Push-As-Ref($namespace, $tag)
{
    $ref = "${Env:CI_COMMIT_TAG}"
    if ($ref -eq "") {
       $ref = "${Env:CI_COMMIT_REF_SLUG}"
    }
    if($ref -eq "") {
        $ref = "main"
    }
    if($ref -eq "main") {
        $ref = "bleeding"
    }
    Push-As $namespace $tag $ref
}

function Push-As($namespace, $tag, $alias)
{
    $image = "${namespace}/gitlab-runner-helper:$tag"

    $newTag = "x86_64-${alias}-$Env:WINDOWS_VERSION"
    $newImage = "${namespace}/gitlab-runner-helper:$newTag"

    Write-Information "Tag $tag as $newTag"
    & 'docker' tag $image $newImage
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to tag $tag as $newTag" )
    }

    Write-Information "Push image $newImage"
    & 'docker' push $newImage
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to push image $newImage to registry" )
    }
}

function Push-Latest($namespace)
{
    Write-Information "Push latest tag"

    & 'docker' push "${namespace}/gitlab-runner-helper:x86_64-latest-$Env:WINDOWS_VERSION"
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to push image to registry" )
    }
}

function Connect-Registry($username, $password, $registry)
{
    Write-Information "Login registry $registry"

    & 'docker' login --username $username --password $password $registry
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to login Docker hub" )
    }
}

function Disconnect-Registry($registry)
{
    Write-Information "Logout registry $registry"

    & 'docker' logout $registry
    if ($LASTEXITCODE -ne 0) {
        throw ("Failed to logout from Docker hub" )
    }
}

function DockerHub-Namespace
{
    if(-not (Test-Path env:DOCKER_HUB_NAMESPACE))
    {
        return "gitlab"
    }

    return $Env:DOCKER_HUB_NAMESPACE
}

function ECR-Public-Registry
{
    if(-not (Test-Path env:ECR_PUBLIC_REGISTRY))
    {
        return "public.ecr.aws/gitlab"
    }

    return $Env:ECR_PUBLIC_REGISTRY
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
        $dockerHubNamespace = DockerHub-Namespace

        # We don't really care if these fail or not, clean up shouldn't fail
        # the pipelines.
        & 'docker' rmi -f $Env:CI_REGISTRY_IMAGE/gitlab-runner-helper:$tag
        & 'docker' rmi -f $dockerHubNamespace/gitlab-runner-helper:$tag
        & 'docker' image prune -f
    }
}
