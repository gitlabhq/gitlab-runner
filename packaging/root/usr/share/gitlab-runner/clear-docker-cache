#!/usr/bin/env bash
# http://redsymbol.net/articles/unofficial-bash-strict-mode/

#########################################################################################
#  SCRIPT: clear-docker-cache.sh
#  Description: Used to cleanup unused docker containers and volumes
######################################################################################
IFS=$'\n\t'
set -euo pipefail

if ! [ -x "$(command -v docker)" ]; then
    echo -e "INFO: Docker installation not found, skipping clear-docker-cache"
    exit 0
fi

DOCKER_VERSION=$(docker version --format '{{.Server.Version}}') #get docker version
REQUIRED_DOCKER_VERSION=1.13

#print usage information
usage() {
   echo -e "\nUsage: $0 prune-volumes|prune|space|help\n"
   echo -e "\tprune-volumes    Remove all unused containers (both dangling and unreferenced) and volumes"
   echo -e "\tprune            Remove all unused containers (both dangling and unreferenced)"
   echo -e "\tspace            Show docker disk usage"
   echo -e "\thelp             Show usage"
   exit 1 # Exit script after printing usage
}

if  awk 'BEGIN {exit !('$DOCKER_VERSION' < '$REQUIRED_DOCKER_VERSION')}'; then
    echo -e "\nERROR: Your current API version is lower than 1.25. The client and daemon API must both be at least 1.25+ to run these commands. Kindly upgrade your docker version\n"
    exit 1
fi


COMMAND="${1:-prune-volumes}"

case "$COMMAND" in

  prune)

    echo -e "\nCheck and remove all unused containers (both dangling and unreferenced)"
    echo -e "-----------------------------------------------------------------------\n\n"
    docker system prune -af --filter label=com.gitlab.gitlab-runner.managed=true

    exit 0
    ;;

  space)

    echo -e "\nShow docker disk usage"
    echo -e "----------------------\n"
    docker system df

    exit 0
    ;;

  help)

    usage
    ;;

  prune-volumes)

    echo -e "\nCheck and remove all unused containers (both dangling and unreferenced) including volumes."
    echo -e "------------------------------------------------------------------------------------------\n\n"
    docker system prune --volumes -af --filter label=com.gitlab.gitlab-runner.managed=true

    exit 0
    ;;

esac
