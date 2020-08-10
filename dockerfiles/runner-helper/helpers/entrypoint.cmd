@echo off

IF NOT DEFINED LOCAL_CA_PATH (SET LOCAL_CA_PATH="C:\GitLab-Runner\certs\ca.crt")

IF EXIST %LOCAL_CA_PATH% (
    echo "Adding CA certificate..."
    certutil -addstore "Root" %LOCAL_CA_PATH%
)

%*
