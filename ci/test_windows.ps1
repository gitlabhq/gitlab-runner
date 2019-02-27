param(
    [string]$testsDefinitionsFile = ".\testsdefinitions.txt"
)

$InformationPreference = "Continue"

function Get-Line([string]$file) {
    (Get-Content $file | Measure-Object -Line).Lines
}

$numberOfDefinitions = Get-Line -file $testsDefinitionsFile
$executionSize = [int]($numberOfDefinitions / $env:CI_NODE_TOTAL)
$nodeIndex = $env:CI_NODE_INDEX - 1
$executionOffset = ($nodeIndex * $executionSize)

Write-Information "Number of definitions: $numberOfDefinitions"
Write-Information "Suite size: $env:CI_NODE_TOTAL"
Write-Information "Suite index: $env:CI_NODE_INDEX"

Write-Information "Execution size: $executionSize"
Write-Information "Execution offset: $executionOffset"

$failed = @()
Get-Content $testsDefinitionsFile | Select-Object -skip $executionOffset -first $executionSize | ForEach-Object {
    $pkg, $index, $tests = $_.Split(" ", 3)

    Write-Information "`r`n`r`n--- Starting part $index of go tests of '$pkg' package:`r`n`r`n"

    go test -v $pkg -run "$tests"

    if ($LASTEXITCODE -ne 0) {
        $failed += "$pkg-$index"
    }
}

if ($failed.count -ne 0) {
    Write-Output ""
    Write-Warning "Failed packages:"
    $failed | Out-String | Write-Warning

    exit 1
}
