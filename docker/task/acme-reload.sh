#!/bin/sh

reload_nginx() {
    echo "Reloading Nginx configuration..."
    nginx -s reload
    if [ $? -ne 0 ]; then
        echo "Failed to reload Nginx configuration."
    fi
}

reload_nginx