# mediaproxy

`mediaproxy` is a high-performance, low-resource media proxy written in Go. It intelligently caches and optimizes images, while efficiently streaming video and audio to offload traffic from your origin servers.

## Features

- **Intelligent**: Optimizes static images to WebP, preserves GIF animations, and streams video/audio.
- **High-Performance**: Uses an in-memory cache (ristretto) for instant delivery of hot assets.
- **Low Usage**: Built with Go and `libvips` for minimal CPU and memory footprint.
- **Whitelisting**: Optionally restrict proxying to a specific list of allowed domains.
- **Configurable**: All settings are managed via environment variables for easy deployment.

## Example Usage

The service endpoint is `https://<your-domain>/<full_media_url>`.

## Configuration

The service is configured using environment variables:

| Variable              | Description                                                  | Default          |
| --------------------- | ------------------------------------------------------------ | ---------------- |
| `LOG_LEVEL`           | The logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`).        | `INFO`           |
| `CACHE_TTL`           | The duration for which images are cached.                    | `10m`            |
| `ALLOWED_DOMAINS`     | Comma-separated list of domains to whitelist.                | (empty, all allowed) |
| `MAX_ALLOWED_SIZE`    | Maximum media file size in bytes.                            | `52428800` (50MB)|
| `DEFAULT_IMAGE_QUALITY` | Quality for optimized WebP images (1-100).                 | `80`             |
| `CLIENT_TIMEOUT`      | Timeout for fetching media from the origin.                  | `2m`             |

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

Create a `compose.yaml` file with the following content:

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

## License

[GPL-3.0](https://github.com/skidoodle/mediaproxy/blob/main/license)
