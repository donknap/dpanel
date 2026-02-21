#!/bin/sh

NGINX_CONFIG_DIR="/dpanel/nginx"

chmod 755 /app/server/dpanel && mkdir -p /dpanel/nginx/extra_host /dpanel/nginx/proxy_host /dpanel/nginx/temp /dpanel/cert /dpanel/storage

if command -v crond >/dev/null 2>&1; then
    crond
elif command -v cron >/dev/null 2>&1; then
    service cron start
fi

mkdir -p /var/log/nginx && nginx -g "daemon off;" &
/app/server/dpanel server:start -f /app/server/config.yaml