# Cache

## S3 Cache

> [!NOTE]
> The S3 cache **does not replace** the other required cache configuration.
> You must still configure options like `.cache.oci.*` and `.cache.signingKeyPath`.

SeaweedFS Setup (Local S3 Emulation).

SeaweedFS serves S3 on port 8333, and takes its S3 credentials from a config file:
without one it serves anonymously.

Example `s3.json`:

```json
{
  "identities": [
    {
      "name": "image-factory",
      "credentials": [
        {
          "accessKey": "AKIA6Z4C7N3S2JD3JH9A",
          "secretKey": "y1rE4xZnqO6xvM7L0jFD3EXAMPLEnG4K2vOfLp8Iv9"
        }
      ],
      "actions": ["Admin", "Read", "Write", "List", "Tagging"]
    }
  ]
}
```

Example `docker-compose.yaml` snippet:

```yaml
services:
  seaweedfs:
    image: chrislusf/seaweedfs:4.47
    container_name: seaweedfs_local
    ports:
      - "9000:8333"
    volumes:
      - ${PWD}/data:/data
      - ${PWD}/config/s3.json:/etc/seaweedfs/s3.json:ro
    command: server -dir=/data -s3 -s3.port=8333 -s3.config=/etc/seaweedfs/s3.json
    restart: unless-stopped
```

The `image-factory` bucket must exist before starting Image Factory:

```shell
docker exec -i seaweedfs_local weed shell -master=localhost:9333 <<<"s3.bucket.create -name image-factory"
```

Environment Variables:

```env
AWS_ACCESS_KEY_ID=AKIA6Z4C7N3S2JD3JH9A
AWS_SECRET_ACCESS_KEY=y1rE4xZnqO6xvM7L0jFD3EXAMPLEnG4K2vOfLp8Iv9
```

Example Image Factory config snippet:

```yaml
cache:
  s3:
    # Enable S3 cache for boot assets
    enabled: true
  
    # S3 bucket name, it must exist before starting Image Factory
    bucket: image-factory
  
    # S3 endpoint
    endpoint: localhost:9000
  
    # (optional) S3 region
    region: eu-central-1
```

## CDN Cache

> [!NOTE]
> The CDN cache is an **overlay** - it requires the S3 cache to be enabled.

Emulating a CDN with Nginx.

Example `docker-compose.yaml` snippet:

```yaml
services:
  nginx:
    image: nginx
    container_name: nginx_redirect
    ports:
      - "3000:80"
    volumes:
      - ./config/nginx.conf:/etc/nginx/conf.d/default.conf:ro
```

Example Nginx Configuration:

```nginx
server {
    listen 80;

    location /health {
        return 200 'OK';
        add_header Content-Type text/plain;
    }

    location / {
        return 307 http://localhost:9000/image-factory$request_uri;
    }
}
```

Example Image Factory config snippet:

```yaml
cache:
  cdn:
    # Enable CDN for boot assets
    enabled: true
  
    # CDN host to replace from presigned S3 URL
    host: localhost:3000
  
    # Path prefix to strip from S3 presigned URL, when redirecting CDN
    trimPrefix: /image-factory

    # Shared secret used to sign CDN URLs with a timed-HMAC token
    hmacSecretPath: /etc/image-factory/cdn-hmac-secret
```

### Signing CDN URLs

A CDN hostname in front of object storage does not validate the presigned signature the
storage provider generated: the redirect target is readable by anyone who knows the object
key, with no expiry.
Setting `cache.cdn.hmacSecretPath` appends a timed-HMAC token to every CDN redirect:

```text
?...&verify=<unix-seconds>-<base64url(HMAC-SHA256(secret, message + unix-seconds))>
```

`message` is the whole request URI ahead of the token — the path after the host rewrite and
prefix trim, plus the presigned query string.
The token is always the last parameter, and uses the URL-safe base64 alphabet with no padding.

Pair it with a WAF rule that blocks requests failing HMAC validation,
using the same secret.

The registry serving installer images needs the equivalent `cfhmac` storage middleware,
otherwise `crane pull` breaks while asset downloads keep working.

Leave `hmacSecretPath` unset to keep the previous behaviour.
