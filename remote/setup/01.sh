#!/bin/bash
set -eu

USERNAME=deploy
PROJECT_DIR=/home/${USERNAME}/mpp-viewer

read -p "Enter the server repository URL: " SERVER_REPO
read -p "Enter the parser repository URL: " PARSER_REPO

export LC_ALL=en_US.UTF-8

apt update
apt --yes install ca-certificates curl git rsync

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" \
    > /etc/apt/sources.list.d/docker.list

apt update
apt --yes install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable --now docker

useradd --create-home --shell "/bin/bash" --groups sudo,docker "${USERNAME}"
passwd --delete "${USERNAME}"
chage --lastday 0 "${USERNAME}"
rsync --archive --chown=${USERNAME}:${USERNAME} /root/.ssh /home/${USERNAME}

sudo -u "${USERNAME}" mkdir -p "${PROJECT_DIR}"
sudo -u "${USERNAME}" git clone "${SERVER_REPO}" "${PROJECT_DIR}/server"
sudo -u "${USERNAME}" git clone "${PARSER_REPO}" "${PROJECT_DIR}/parser"

docker --version
docker compose version

echo "Setup complete. Write ${PROJECT_DIR}/server/.env, then bring the stack up."
