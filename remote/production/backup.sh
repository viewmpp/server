#!/usr/bin/env bash
set -euo pipefail

CONTAINER=${CONTAINER:-server-db}
BACKUP_DIR=${BACKUP_DIR:-/var/backups/viewmpp}
KEEP=${KEEP:-30}
FREE_FLOOR_MB=${FREE_FLOOR_MB:-1024}

log() {
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

die() {
  log "failed: $*" >&2
  exit 1
}

[ -d "$BACKUP_DIR" ] || die "$BACKUP_DIR does not exist"
[ -w "$BACKUP_DIR" ] || die "$BACKUP_DIR is not writable by $(id -un)"

free_mb=$(df -Pm "$BACKUP_DIR" | awk 'NR == 2 { print $4 }')
[ "$free_mb" -ge "$FREE_FLOOR_MB" ] ||
  die "only ${free_mb}MB free where backups live, ${FREE_FLOOR_MB}MB is the floor"

[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || echo false)" = true ] ||
  die "container $CONTAINER is not running"

docker exec "$CONTAINER" sh -c 'pg_isready -q -U "$POSTGRES_USER" -d "$POSTGRES_DB"' ||
  die "postgres in $CONTAINER is not accepting connections"

stamp=$(date -u +%Y-%m-%dT%H%M%SZ)
target="$BACKUP_DIR/viewmpp-$stamp.dump"
partial="$target.part"

umask 077
trap 'rm -f "$partial"' EXIT

docker exec "$CONTAINER" sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --encoding=UTF8 --no-owner --no-privileges' \
  > "$partial" || die "pg_dump did not finish"

[ -s "$partial" ] || die "pg_dump produced an empty file"

docker exec -i "$CONTAINER" pg_restore --list > /dev/null < "$partial" ||
  die "the dump did not survive its own integrity check"

mv "$partial" "$target"
chmod 600 "$target"

log "wrote $target ($(du -h "$target" | cut -f1))"

find "$BACKUP_DIR" -maxdepth 1 -type f -name 'viewmpp-*.dump' -print |
  sort |
  head -n "-$KEEP" |
  while IFS= read -r stale; do
    rm -f -- "$stale"
    log "removed $stale"
  done

kept=$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'viewmpp-*.dump' | wc -l)
log "done, $kept copies kept"
