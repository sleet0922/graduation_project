#!/bin/sh
set -eu
export PGDATA=/var/lib/postgresql/17/data
mkdir -p /run/postgresql
chown postgres:postgres /run/postgresql
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  mkdir -p /var/lib/postgresql/17/data
  chown -R postgres:postgres /var/lib/postgresql
  su postgres -s /bin/sh -c "initdb -D '$PGDATA' --auth=trust"
  su postgres -s /bin/sh -c "pg_ctl -D '$PGDATA' -o '-c listen_addresses=127.0.0.1' -w start"
  su postgres -s /bin/sh -c "psql -v ON_ERROR_STOP=1 -d postgres -c \"CREATE ROLE sleet LOGIN PASSWORD 'Zyz20050922!'\""
  su postgres -s /bin/sh -c "createdb -O sleet graduation_project"
else
  su postgres -s /bin/sh -c "pg_ctl -D '$PGDATA' -w start"
fi
redis-server --daemonize yes
livekit-server --config /etc/livekit/livekit.yaml >/var/log/livekit.log 2>&1 &
cd /opt/graduation-project
ZAT_DATABASE_PASSWORD='Zyz20050922!' ZAT_DATABASE_AUTO_MIGRATE=true /usr/local/bin/graduation-project >/var/log/graduation-project/app.log 2>&1 &
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
