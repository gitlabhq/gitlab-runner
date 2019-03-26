param(
    [string]$TargetFile,
    [string]$ExpectedHash
)

$hash = Get-FileHash -Path $TargetFile -Algorithm SHA256

if (-not ($hash.Hash -eq $ExpectedHash)) {
    Write-Warning "SHA256 checksum for $TargetFile is invalid"
    exit 1
}

Write-Output "SHA256 checksum for $TargetFile is valid"
exit 0
