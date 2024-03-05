#!/bin/sh

NGINX_CONFIG_DIR="/dpanel/nginx"
NGINX_CMD="nginx"

reload_nginx() {
    echo "Reloading Nginx configuration..."
    $NGINX_CMD -s reload
    if [ $? -ne 0 ]; then
        echo "Failed to reload Nginx configuration."
    fi
}

while true; do
    inotifywait -r -e modify,create,delete,move "$NGINX_CONFIG_DIR"
    reload_nginx
done &

nginx -g 'daemon off;'