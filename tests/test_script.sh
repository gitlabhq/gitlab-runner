#!/bin/bash

set -e
USER="$1"

status() {
	pidof gitlab-runner
}

echo Checking existence of "$USER"...
id -u "$USER"

echo Check if /etc/gitlab-runner/config.toml is created...
if [[ -f /etc/gitlab-runner/config.toml ]]; then
	CONFIG=$(stat -c "%A %U %G" /etc/gitlab-runner/config.toml)
	echo "$CONFIG" | grep "^-rw-------"
	echo "$CONFIG" | grep "root root"
fi

echo List of processes:
ps auxf
echo

echo Checking if runner is running...
status
echo

echo Testing help...
gitlab-runner --help > /dev/null
echo

echo Stopping runner...
gitlab-runner stop
status && exit 1
echo

echo Starting runner...
gitlab-runner start
sleep 1s
status
echo

echo Checking su...
echo id | su --shell /bin/bash --login "$USER"
