#!/bin/sh
# Drops from root to a uid/gid matching the host's media library ownership
# (PUID/PGID), so the container can write to bind-mounted /movies and /tv
# without the operator having to chown their whole library to a fixed uid.
#
# The target id may already belong to a built-in Alpine account (e.g. gid 100
# is "users", uid 99 is "nobody") — reuse it by name instead of trying to
# create a colliding one.
set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

group_name="$(awk -F: -v gid="$PGID" '$3==gid {print $1; exit}' /etc/group)"
if [ -z "$group_name" ]; then
    delgroup reclaim 2>/dev/null || true
    addgroup -g "$PGID" reclaim
    group_name=reclaim
fi

user_name="$(awk -F: -v uid="$PUID" '$3==uid {print $1; exit}' /etc/passwd)"
if [ -z "$user_name" ]; then
    deluser reclaim 2>/dev/null || true
    adduser -D -H -u "$PUID" -G "$group_name" reclaim
    user_name=reclaim
fi

mkdir -p /data
chown -R "$user_name:$group_name" /data

exec su-exec "$user_name:$group_name" /reclaim
