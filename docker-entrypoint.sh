#!/bin/sh
# The container starts as root so it can fix ownership of whatever is
# mounted at /data - a fresh bind-mount from a git checkout is owned by
# whoever ran `git clone` (usually root), not the app's non-root user, and a
# brand-new named volume starts out root-owned too. Without this, a plain
# `docker compose up` on a clean checkout fails with "permission denied"
# the moment the app tries to create kkt.db. Once ownership is fixed,
# privileges are dropped to the unprivileged "kkt" user before running the
# actual binary.
set -e

if [ "$(id -u)" = "0" ]; then
	chown -R kkt:kkt /data
	exec su-exec kkt "$@"
fi

exec "$@"
