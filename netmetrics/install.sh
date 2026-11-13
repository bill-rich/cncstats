#!/bin/bash
# Installs or updates netmetrics + Grafana on cncstats-east. Run as root from
# a copy of this directory:  sudo bash install.sh
# Safe to rerun: keeps the existing passwords, DB rows and Grafana state.
set -euo pipefail
SRC=$(cd "$(dirname "$0")" && pwd)
DST=/opt/netmetrics
ENV=$DST/grafana.env
GRAFANA_IMAGE=grafana/grafana:11.4.0

install -d -m 755 $DST
cp -r "$SRC"/netmetrics.py "$SRC"/schema.sql "$SRC"/grafana $DST/
chmod -R go-w $DST

if [ ! -f $ENV ]; then
  umask 077
  cat > $ENV <<EOF
GF_SECURITY_ADMIN_PASSWORD=$(head -c 18 /dev/urandom | base64 | tr -d '/+=')
ZM_RO_PASSWORD=$(head -c 18 /dev/urandom | base64 | tr -d '/+=')
EOF
fi
. $ENV

# Database + a read only login for Grafana. The DB port is published on
# 127.0.0.1 only; Grafana reaches it through the docker proxy, so the source
# address is a docker bridge IP.
docker exec -i gosvc-db sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < $DST/schema.sql
docker exec -i gosvc-db sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' <<EOF
CREATE USER IF NOT EXISTS 'grafana_ro'@'%' IDENTIFIED BY '$ZM_RO_PASSWORD';
ALTER USER 'grafana_ro'@'%' IDENTIFIED BY '$ZM_RO_PASSWORD';
GRANT SELECT, SHOW VIEW ON zulumetrics.* TO 'grafana_ro'@'%';
EOF

cat > /etc/systemd/system/netmetrics.service <<EOF
[Unit]
Description=Load Zulu GO client logs into zulumetrics
After=docker.service
[Service]
Type=oneshot
Nice=10
ExecStart=/usr/bin/python3 -I $DST/netmetrics.py
EOF
cat > /etc/systemd/system/netmetrics.timer <<EOF
[Unit]
Description=Load Zulu GO client logs into zulumetrics every 5 minutes
[Timer]
OnBootSec=2min
OnUnitActiveSec=5min
[Install]
WantedBy=timers.target
EOF
systemctl daemon-reload
systemctl enable --now netmetrics.timer

docker rm -f grafana >/dev/null 2>&1 || true
docker run -d --name grafana --restart unless-stopped --network host --memory 300m \
  -v grafana_data:/var/lib/grafana \
  -v $DST/grafana/provisioning:/etc/grafana/provisioning:ro \
  -v $DST/grafana/dashboards:/var/lib/grafana/dashboards:ro \
  -e GF_SERVER_HTTP_ADDR=127.0.0.1 -e GF_SERVER_HTTP_PORT=3000 \
  -e GF_SERVER_ROOT_URL=https://cncstats.computersrfun.org/grafana/ -e GF_SERVER_SERVE_FROM_SUB_PATH=true \
  -e GF_SECURITY_ADMIN_PASSWORD="$GF_SECURITY_ADMIN_PASSWORD" -e GF_SECURITY_COOKIE_SECURE=true \
  -e GF_USERS_ALLOW_SIGN_UP=false -e GF_AUTH_ANONYMOUS_ENABLED=false \
  -e GF_ANALYTICS_REPORTING_ENABLED=false -e GF_ANALYTICS_CHECK_FOR_UPDATES=false -e GF_NEWS_NEWS_FEED_ENABLED=false \
  -e GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/var/lib/grafana/dashboards/zm-overview.json \
  -e ZM_DB_ADDR=127.0.0.1:3306 -e ZM_RO_PASSWORD="$ZM_RO_PASSWORD" \
  $GRAFANA_IMAGE >/dev/null
echo "grafana up; admin password is in $ENV"
