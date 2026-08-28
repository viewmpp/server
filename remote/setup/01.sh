#!/bin/bash
set -eu

USERNAME=dzenthai
PROJECT_DIR=/viewmpp

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

install -d -m 700 -o "${USERNAME}" -g "${USERNAME}" "${SSH_DIR}"

ssh-keyscan github.com > "${SSH_DIR}/known_hosts" 2> /dev/null
chown "${USERNAME}:${USERNAME}" "${SSH_DIR}/known_hosts"
chmod 600 "${SSH_DIR}/known_hosts"

if [ ! -f "${SSH_DIR}/id_ed25519" ]; then
    sudo -u "${USERNAME}" -H ssh-keygen -q -t ed25519 -N "" -C "${USERNAME}@$(hostname)" -f "${SSH_DIR}/id_ed25519"
fi

echo
cat "${SSH_DIR}/id_ed25519.pub"
echo

read -r -p "Add this key at GitHub, then press Enter: " _

install -d -o "${USERNAME}" -g "${USERNAME}" "${PROJECT_DIR}"

read -r -p "Enter the server repository URL: " SERVER_REPO
read -r -p "Enter the parser repository URL: " PARSER_REPO

sudo -u "${USERNAME}" -H git clone "${SERVER_REPO}" "${PROJECT_DIR}/server"
sudo -u "${USERNAME}" -H git clone "${PARSER_REPO}" "${PROJECT_DIR}/parser"

sed "s|/home/dzenthai/mpp-viewer|${PROJECT_DIR}|g; s|^User=.*|User=${USERNAME}|" \
    "${PROJECT_DIR}/server/remote/production/mpp-backup.service" \
    > /etc/systemd/system/mpp-backup.service
chmod 0644 /etc/systemd/system/mpp-backup.service

install -m 0644 "${PROJECT_DIR}/server/remote/production/mpp-backup.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now mpp-backup.timer