@echo off
IF "%1"=="get" (
    echo {
    echo   "username": "script_user_1",
    echo   "secret": "script_password_1",
    echo   "serveraddress": "https://registry2.domain.tld:5005/v1/"
    echo }
    exit
)
IF "%1"=="list" (
    echo {
    echo   "https://registry2.domain.tld:5005/v1/": "script_user_1"
    echo }
    exit
)
