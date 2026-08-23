#!/bin/bash
set -eu

TIMEZONE=Etc/UTC

USERNAME=mppviewer

PARSER_USERNAME=parser

MIGRATE_VERSION=v4.19.1
POSTGRES_VERSION=17

read -p "Enter the domain this server will serve (e.g. mppviewer.com): " DOMAIN
read -s -p "Enter password for the ${USERNAME} DB user: " DB_PASSWORD
echo
read -p "Enter the Resend API key: " RESEND_API_KEY
read -p "Enter the Resend sender address: " RESEND_SENDER

export LC_ALL=en_US.UTF-8

add-apt-repository --yes universe
apt update

timedatectl set-timezone ${TIMEZONE}
apt --yes install locales-all

apt --yes install curl wget ca-certificates gpg apt-transport-https \
    debian-keyring debian-archive-keyring lsb-release

useradd --create-home --shell "/bin/bash" --groups sudo "${USERNAME}"
passwd --delete "${USERNAME}"
chage --lastday 0 "${USERNAME}"
rsync --archive --chown=${USERNAME}:${USERNAME} /root/.ssh /home/${USERNAME}

useradd --system --create-home --shell "/usr/sbin/nologin" "${PARSER_USERNAME}"

ufw allow 22
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

apt --yes install fail2ban

curl -L "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
    | tar xz -C /usr/local/bin migrate
chmod +x /usr/local/bin/migrate
migrate -version

install -d /usr/share/postgresql-common/pgdg
curl --fail -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    https://www.postgresql.org/media/keys/ACCC4CF8.asc
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list

apt update
apt --yes install postgresql-${POSTGRES_VERSION}

sudo -i -u postgres psql -c "CREATE ROLE ${USERNAME} WITH LOGIN PASSWORD '${DB_PASSWORD}'"
sudo -i -u postgres psql -c "CREATE DATABASE ${USERNAME} OWNER ${USERNAME}"
sudo -i -u postgres psql -d ${USERNAME} -c "CREATE EXTENSION IF NOT EXISTS citext"

install -d /etc/apt/keyrings
wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public \
    | gpg --dearmor -o /etc/apt/keyrings/adoptium.gpg
echo "deb [signed-by=/etc/apt/keyrings/adoptium.gpg] https://packages.adoptium.net/artifactory/deb $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" \
    > /etc/apt/sources.list.d/adoptium.list

apt update
apt --yes install temurin-25-jre

curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    > /etc/apt/sources.list.d/caddy-stable.list

apt update
apt --yes install caddy

cat > /etc/mppviewer.env <<EOF
ENV=prod
PORT=4000
BASE_URL='https://${DOMAIN}'
PROXIES=1
DB_DSN='postgres://${USERNAME}:${DB_PASSWORD}@localhost:5432/${USERNAME}?sslmode=disable'
PARSER_URL='http://127.0.0.1:8080/parse'
RESEND_API_KEY='${RESEND_API_KEY}'
RESEND_SENDER='${RESEND_SENDER}'
EARLY_ACCESS_SEATS=100
EOF

chown root:${USERNAME} /etc/mppviewer.env
chmod 640 /etc/mppviewer.env

apt --yes -o Dpkg::Options::="--force-confnew" upgrade

echo "Script complete! Rebooting..."
reboot
