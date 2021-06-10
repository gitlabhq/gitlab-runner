foreach ($var in get-childitem env:*) {
    switch ($var.name) {
        "SystemRoot" {}
        "HOMEPATH" {}
        "COMPUTERNAME" {}
        "ALLUSERSPROFILE" {}
        "SystemDrive" {}
        "HOMEDRIVE" {}
        "APPDATA" {}
        "USERDOMAIN" {}
        "LOCALAPPDATA" {}
        default {
            $value = [Environment]::GetEnvironmentVariable($var.name, 'User')
            if (!$value) {
                $value = [Environment]::GetEnvironmentVariable($var.name, 'Machine')
                if (!$value) {
                    remove-item "env:$($var.name)"
                }
            }
        }
    }
}

go test $args