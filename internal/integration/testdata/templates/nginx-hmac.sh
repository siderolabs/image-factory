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
        # stands in for the Cloudflare WAF rule: the token must be present
        if ($arg_verify = "") {
            return 403;
        }
        # the S3 origin does validate SigV4, so the token comes back off before forwarding
        set $fwd $args;
        if ($fwd ~ "^(.*)&verify=[^&]*$") {
            set $fwd $1;
        }
        if ($fwd ~ "^(.*)&verify=[^&]*&(.*)$") {
            set $fwd "$1&$2";
        }
        if ($fwd ~ "^verify=[^&]*&(.*)$") {
            set $fwd $1;
        }
        return 307 http://%s/%s$uri?$fwd;
    }
}
EOF

./docker-entrypoint.sh nginx -g 'daemon off;'
