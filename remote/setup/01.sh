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

install -m 0644 "${PROJECT_DIR}/server/remote/production/mpp-backup.service" /etc/systemd/system/
install -m 0644 "${PROJECT_DIR}/server/remote/production/mpp-backup.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now mpp-backup.timer

cat <<MSG

Setup complete. Two steps left.

1. Write ${PROJECT_DIR}/server/.env with at least:

     ENV=prod
     BASE_URL=https://<your domain>
     DOMAIN=<your domain>
     ACME_EMAIL=<address for Let's Encrypt>
     POSTGRES_USER=... POSTGRES_PASSWORD=... POSTGRES_DB=...
     RESEND_API_KEY=... RESEND_SENDER=noreply@<your domain>

2. Bring the stack up with both compose files:

     cd ${PROJECT_DIR}/server
     docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up -d --build

   The base file alone publishes no ports - Caddy lives in the prod file and is the
   only way in. To update later, git pull in both repos and repeat the same command.

3. Take the first backup by hand and restore it into a scratch database. A backup
   nobody has restored is not yet a backup:

     systemctl start mpp-backup.service
     journalctl -u mpp-backup.service -n 20

     docker compose -f docker-compose.yaml -f docker-compose.prod.yaml exec -T server-db \\
       pg_restore -U "\$POSTGRES_USER" -d "\$POSTGRES_DB" --clean --if-exists < ../backups/<file>.dump

   Dumps land in ${PROJECT_DIR}/backups, 14 days are kept, the timer runs nightly at
   03:30 UTC. They sit on the same disk as the database - set BACKUP_REMOTE in the
   unit to mirror them off the machine.
MSG
