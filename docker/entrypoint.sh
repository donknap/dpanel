#!/bin/sh

NGINX_CONFIG_DIR="/dpanel/nginx"

[ -e /var/run/dpanel/dpanel.sock ] && rm -f /var/run/dpanel/dpanel.sock

chmod 755 /app/server/dpanel && mkdir -p /dpanel/nginx/default_host /dpanel/nginx/proxy_host \
  /dpanel/nginx/redirection_host /dpanel/nginx/dead_host /dpanel/nginx/temp \
  /dpanel/cert /dpanel/storage

crond
nginx -g "daemon off;" &
/app/server/dpanel server:start -f /app/server/config.yaml
