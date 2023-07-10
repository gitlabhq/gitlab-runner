#!/bin/bash

# gitlab-runner or gitlab-runner-helper
IMAGE="$1"
NAME="$2"

API_RESULT=$(curl -s "https://hub.docker.com/v2/repositories/gitlab/$IMAGE/tags?page_size=100&name=$NAME")
NAMES=$(echo "$API_RESULT" | jq -r .results[].name)

echo "$NAMES"
