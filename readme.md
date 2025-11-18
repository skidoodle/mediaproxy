# mediaproxy

`mediaproxy` is a high-performance, low-resource media proxy written in Go. It intelligently caches and optimizes images, while efficiently streaming video and audio to offload traffic from your origin servers.

## Features

- **Intelligent**: Optimizes static images to WebP, preserves GIF animations, and streams video/audio.
- **High-Performance**: Uses an in-memory cache (ristretto) for instant delivery of hot assets.
- **Low Usage**: Built with Go and `libvips` for minimal CPU and memory footprint.
- **Flexible Modes**: Can be used as a standard proxy requiring the full media URL or as a direct middleware with a pre-configured base URL.
- **Whitelisting**: In standard mode, you can restrict proxying to a specific list of allowed domains.
- **Configurable**: All settings are managed via environment variables for easy deployment.

## Example Usage

`mediaproxy` can run in two modes:

### Standard Mode

In this mode, you pass the full URL of the media asset in the path.

The service endpoint is `https://<your-domain>/<full_media_url>`.

**Example:** `https://cdn.example.com/https://images.pexels.com/photos/1314550/pexels-photo-1314550.jpeg`

### Middleware Mode

This mode is activated by setting the `BASE_URL` environment variable. It allows you to use the service as a direct proxy without passing the full URL.

**Example:**

1. Set the environment variable `BASE_URL=cdn.example.com`.
2. Request `https://media.example.com/object/image.jpg`.
3. The service will proxy the request to `https://cdn.example.com/object/image.jpg`.

This is useful for cleaner URLs and when proxying to a single, trusted domain.

## Configuration

The service is configured using environment variables:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `LOG_LEVEL` | The logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`). | `INFO` |
| `BASE_URL` | If set, activates middleware mode. The service will prepend `https://<BASE_URL>/` to all incoming request paths. When active, `ALLOWED_DOMAINS` is ignored. | (empty, disabled) |
| `CACHE_TTL` | The duration for which images are cached. | `10m` |
| `ALLOWED_DOMAINS` | Comma-separated list of domains to whitelist. Ignored if `BASE_URL` is set. | (empty, all allowed) |
| `MAX_ALLOWED_SIZE` | Maximum media file size in bytes. | `52428800` (50MB)|
| `DEFAULT_IMAGE_QUALITY` | Quality for optimized WebP images (1-100). | `80` |
| `CLIENT_TIMEOUT` | Timeout for fetching media from the origin. | `2m` |

## Running Locally

### With Docker (Recommended)

This is the easiest way to run the service, as it handles the `libvips` dependency automatically.

```sh
git clone https://github.com/skidoodle/mediaproxy
cd mediaproxy
docker compose up -f compose.dev.yaml --build
```

### Without Docker

Requires Go and `libvips` to be installed on your system.

```sh
# Note: You must install libvips first.
# See: https://www.libvips.org/install.html

git clone https://github.com/skidoodle/mediaproxy
cd mediaproxy
go mod tidy
go run .
```

## Deploying

### Docker Compose

Create a `compose.yaml` file with your desired configuration.

**Standard Mode Example:**

```yaml
services:
  mediaproxy:
    image: ghcr.io/skidoodle/mediaproxy:main
    container_name: mediaproxy
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      LOG_LEVEL: INFO
      CACHE_TTL: 1h
      ALLOWED_DOMAINS: images.pexels.com,media.giphy.com,videos.pexels.com
```

**Middleware Mode Example:**

```yaml
services:
  mediaproxy:
    image: ghcr.io/skidoodle/mediaproxy:main
    container_name: mediaproxy
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      LOG_LEVEL: INFO
      CACHE_TTL: 1h
      BASE_URL: cdn.example.com
```

## License

[GPL-3.0](https://github.com/skidoodle/mediaproxy/blob/main/license)
