#!/bin/bash
set -eu

USERNAME=dzenthai
PROJECT_DIR=/opt/viewmpp

SSH_DIR=/home/${USERNAME}/.ssh

read -rsp "Enter password for ${USERNAME}: " PASSWORD
echo

if ! id -u "${USERNAME}" > /dev/null 2>&1; then
    useradd --create-home --shell /bin/bash "${USERNAME}"
fi

echo "${USERNAME}:${PASSWORD}" | chpasswd

usermod --append --groups sudo "${USERNAME}"

curl -fsSL https://get.docker.com | sh

if getent group docker > /dev/null; then
    usermod --append --groups docker "${USERNAME}"
fi

sudo mkdir -p ${PROJECT_DIR}
chown -R "${USERNAME}:${USERNAME}" "${PROJECT_DIR}"