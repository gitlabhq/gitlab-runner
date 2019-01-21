param([string]$testOpts = "")

$failed = @()
go list ./... | Select-String -Pattern "gitlab.com/gitlab-org/gitlab-runner/vendor/" -NotMatch | ForEach-Object {
    $pkg = %{$_ -replace "\r\n", "" }
    $testCmd = "go test $testOpts $pkg"
    Invoke-Expression $testCmd
    if ( $LASTEXITCODE -ne 0 ) { $failed += $pkg }
}

if ($failed.count -ne 0) {
    Write-Host "Failed packages"
    $failed | Out-String | Write-Host

    exit 1
}
