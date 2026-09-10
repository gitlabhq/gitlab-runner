@echo off
IF "%1"=="list" (
    echo {
    echo   "https://registry2.domain.tld:5005/v1/": "script_user_1"
    echo }
    exit
)
echo get is not supported by this helper 1>&2
exit 1
