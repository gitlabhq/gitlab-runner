$ErrorActionPreference = "Stop"

# Bootstrap golang runner
$godir_parent = "$env:GOPATH/src/gitlab.com/gitlab-org"
New-Item -Type Directory -Force -Path $godir_parent
# We need to replace / -> \ because of mklink
$godir = "$godir_parent/gitlab-runner" -replace '/', '\'
$source = $CI_PROJECT_DIR -replace '/', '\'
if (Test-Path $godir) { cmd /c rmdir $godir }
# Copy-Item -Path . -Destination $godir -Recurse
cmd /c mklink /D $godir $source
cd $godir
