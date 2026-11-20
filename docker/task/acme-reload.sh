#!/bin/sh

echo "Reloading Nginx configuration..."
exec nginx -s reload
