#!/bin/sh
# Drops from root to a uid/gid matching the host's media library ownership
# (PUID/PGID), so the container can write to bind-mounted /movies and /tv
# without the operator having to chown their whole library to a fixed uid.
set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$(id -g reclaim 2>/dev/null)" != "$PGID" ]; then
    deluser reclaim 2>/dev/null || true
    delgroup reclaim 2>/dev/null || true
    addgroup -g "$PGID" reclaim
    adduser -D -H -u "$PUID" -G reclaim reclaim
elif [ "$(id -u reclaim)" != "$PUID" ]; then
    deluser reclaim
    adduser -D -H -u "$PUID" -G reclaim reclaim
fi

chown -R reclaim:reclaim /data

exec su-exec reclaim:reclaim /reclaim
