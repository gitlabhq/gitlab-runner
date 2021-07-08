foreach ($var in get-childitem env:*) {
    [bool] $found = $false
    foreach ($name in get-content ./scripts/envs/allowlist_common.env, ./scripts/envs/allowlist_windows.env) {
        if ($var.name -eq $name) {
            $found = $true
            break
        }
    }

    if (!$found) {
        remove-item "env:$($var.name)"
    }
}

go test $args
