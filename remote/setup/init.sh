#!/bin/bash
set -eu

USERNAME=dzenthai
PROJECT_DIR=/viewmpp

read -rsp "Enter password for ${USERNAME}: " PASSWORD
echo

if ! id -u "${USERNAME}" > /dev/null 2>&1; then
    useradd --create-home --shell /bin/bash "${USERNAME}"
fi

echo "${USERNAME}:${PASSWORD}" | chpasswd

usermod --append --groups sudo "${USERNAME}"

if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
fi

if getent group docker > /dev/null; then
    usermod --append --groups docker "${USERNAME}"
fi

sudo mkdir -p ${PROJECT_DIR}
chown -R "${USERNAME}:${USERNAME}" "${PROJECT_DIR}"