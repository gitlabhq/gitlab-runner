# modify git files to reflect their last change date
git ls-files | ForEach-Object {
    $commit_date = git log -1 --format=%cd --date=iso-strict $_
    (Get-Item $_).LastWriteTime = [DateTime]::Parse($commit_date)
}
