param(
    [string]$testsDefinitionsFile = ".\testsdefinitions.txt"
)

$InformationPreference = "Continue"

function Get-Line([string]$file) {
    (Get-Content $file | Measure-Object -Line).Lines
}

$numberOfDefinitions = Get-Line -file $testsDefinitionsFile
$executionSize = [math]::ceiling($numberOfDefinitions / $env:CI_NODE_TOTAL)
$nodeIndex = $env:CI_NODE_INDEX - 1
$executionOffset = ($nodeIndex * $executionSize)

$revision = ((git rev-parse --short=8 HEAD) | Out-String).TrimEnd()
$goLdflags = "-X gitlab.com/gitlab-org/gitlab-runner/common.REVISION=$revision"

Write-Information "Number of definitions: $numberOfDefinitions"
Write-Information "Suite size: $env:CI_NODE_TOTAL"
Write-Information "Suite index: $env:CI_NODE_INDEX"

Write-Information "Execution size: $executionSize"
Write-Information "Execution offset: $executionOffset"

$type="integration"
if ($env:TESTFLAGS.Contains('!integration')) {
    $type="unit"
}

New-Item -ItemType "directory" -Path ".\" -Name ".testoutput\${type}"

$failed = @()
Get-Content $testsDefinitionsFile | Select-Object -skip $executionOffset -first $executionSize | ForEach-Object {
    $pkg, $index, $tests = $_.Split(" ", 3)
    $pkgSlug = ((Write-Output $pkg | ForEach-Object { $_ -replace "[^a-z0-9_]","_" }))

    Write-Information "`r`n`r`n--- Starting part $index of go $type tests of '$pkg' package:`r`n`r`n"

    powershell -File .\scripts\go_test_no_env.ps1 $env:TESTFLAGS -timeout 30m -v $pkg -ldflags "$goLdflags" -run "$tests" | Tee ".testoutput/${type}/${pkgSlug}.${index}.windows.${WINDOWS_VERSION}.output.txt"

    if ($LASTEXITCODE -ne 0) {
        $failed += "$pkg-$index"
    }
}

if ($failed.count -ne 0) {
    Write-Output ""
    Write-Warning "Failed packages:"
    $failed | Out-String | Write-Warning

    exit 99
}
