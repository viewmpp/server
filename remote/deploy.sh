#!/bin/bash
set -eu

USERNAME=dzenthai
PROJECT_DIR=/viewmpp

read -r -p "Enter the server address: " SSH_ADDRESS

rsync --archive remote/setup/01.sh "${SSH_ADDRESS}:"

if ssh "${SSH_ADDRESS}" test -d "${PROJECT_DIR}/server"; then
    rsync --archive --chmod=F600 --chown="${USERNAME}:${USERNAME}" \
        .env.prod "${SSH_ADDRESS}:${PROJECT_DIR}/server/.env"
else
    echo "run ./01.sh on the server first, then repeat this script"
fi
