#!/usr/bin/env pwsh

# $ErrorActionPreference = "Stop" # Similar to set -e
Set-StrictMode -Version Latest # Similar to set -u

$LOOP_ITERATIONS = if ($env:LOOP_ITERATIONS) { [int]$env:LOOP_ITERATIONS } else { 30 }

function Write-Stderr {
    param ([string]$Message)
    [Console]::Error.WriteLine($Message)
}

function Pre {
    $counter = $LOOP_ITERATIONS
    while ($counter-- -gt 0) {
        Write-Output "[entrypoint][pre][stdout][$counter/$LOOP_ITERATIONS] some pre message on stdout"
        Write-Stderr "[entrypoint][pre][stderr][$counter/$LOOP_ITERATIONS] some pre message on stderr"
        Start-Sleep -Seconds 1
    }

    Write-Stderr "----[ CMD ]---->"
}

function Post {
    Write-Stderr "----[ CMD ]----<"

    $counter = $LOOP_ITERATIONS
    while ($counter-- -gt 0) {
        Write-Output "[entrypoint][post][stdout][$counter/$LOOP_ITERATIONS] some post message on stdout"
        Write-Stderr "[entrypoint][post][stderr][$counter/$LOOP_ITERATIONS] some post message on stderr"
        Start-Sleep -Seconds 1
    }
}

# Main execution
$exitCode = 0
try {
    try {
        Pre
    }
    catch {
        Write-Stderr "Error in Pre: $_"
        # Continue execution even if Pre fails
    }

    # Execute the script arguments
    if ($args.Count -gt 0) {
        $scriptBlock = [ScriptBlock]::Create($args -join ' ')
        & $scriptBlock
        $exitCode = if ($?) { 0 } else { 1 }
    }
}
finally {
    # Call Post in the finally block to ensure it runs even if there were errors
    Post
}

exit $exitCode
