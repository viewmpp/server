#!/bin/bash
set -eu

USERNAME=dzenthai
PROJECT_DIR=/viewmpp

read -r -p "Enter the server address: " SSH_ADDRESS

rsync --archive remote/setup/01.sh "${SSH_ADDRESS}:"