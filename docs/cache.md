# Cache

## S3 Cache

> [!NOTE]
> The S3 cache **does not replace** the other required cache configuration.
> You must still configure options like `.cache.oci.*` and `.cache.signingKeyPath`.

MinIO Setup (Local S3 Emulation).

Example `docker-compose.yaml` snippet:

```yaml
services:
  minio:
    image: minio/minio
    container_name: minio_local
    network_mode: host
    volumes:
      - ${PWD}/data:/mnt/data
    environment:
      MINIO_ROOT_USER: AKIA6Z4C7N3S2JD3JH9A
      MINIO_ROOT_PASSWORD: y1rE4xZnqO6xvM7L0jFD3EXAMPLEnG4K2vOfLp8Iv9
    command: server --console-address ":9001" /mnt/data
    restart: unless-stopped
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
```
