#!/bin/bash
set -euo pipefail

PROJECT_DIR=${PROJECT_DIR:-/home/deploy/mpp-viewer}
SERVER_DIR=${SERVER_DIR:-${PROJECT_DIR}/server}
BACKUP_DIR=${BACKUP_DIR:-${PROJECT_DIR}/backups}
KEEP_DAYS=${KEEP_DAYS:-14}

cd "${SERVER_DIR}"

set -a
. ./.env
set +a

: "${POSTGRES_USER:?POSTGRES_USER missing from .env}"
: "${POSTGRES_DB:?POSTGRES_DB missing from .env}"

mkdir -p "${BACKUP_DIR}"

STAMP=$(date --utc +%Y%m%dT%H%M%SZ)
TARGET="${BACKUP_DIR}/${POSTGRES_DB}-${STAMP}.dump"
PARTIAL="${TARGET}.part"

trap 'rm -f "${PARTIAL}"' EXIT

compose() {
    docker compose -f docker-compose.yaml -f docker-compose.prod.yaml "$@"
}

compose exec -T server-db \
    pg_dump --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" --format=custom \
    > "${PARTIAL}"

SIZE=$(stat --format=%s "${PARTIAL}")
if [ "${SIZE}" -lt 1024 ]; then
    echo "dump is suspiciously small (${SIZE} bytes), discarding it" >&2
    exit 1
fi

if ! compose exec -T server-db pg_restore --list < "${PARTIAL}" > /dev/null; then
    echo "dump is unreadable, discarding it and keeping the previous copies" >&2
    exit 1
fi

mv "${PARTIAL}" "${TARGET}"

echo "backup written: ${TARGET} (${SIZE} bytes)"

find "${BACKUP_DIR}" -name "${POSTGRES_DB}-*.dump" -type f -mtime "+${KEEP_DAYS}" -print -delete

if [ -n "${BACKUP_REMOTE:-}" ]; then
    rsync --archive --delete "${BACKUP_DIR}/" "${BACKUP_REMOTE}"
    echo "mirrored to ${BACKUP_REMOTE}"
fi
