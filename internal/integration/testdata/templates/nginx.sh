#!/bin/sh
set -e

cat > /etc/nginx/conf.d/default.conf << 'EOF'
server {
    listen 80;
    location = /health {
        return 200 'OK';
        add_header Content-Type text/plain;
    }
    location / {
        return 307 http://%s/%s$request_uri;
    }
}
EOF

./docker-entrypoint.sh nginx -g 'daemon off;'
