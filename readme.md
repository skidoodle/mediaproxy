# mediaproxy

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Docker Image](https://img.shields.io/badge/Docker-ghcr.io%2Fskidoodle%2Fmediaproxy-blue?style=flat-square&logo=docker)](https://github.com/skidoodle/mediaproxy/pkgs/container/mediaproxy)

A high-performance, low-resource media proxy written in Go. It intelligently caches and optimizes images, while efficiently streaming video and audio to offload traffic from your origin servers.

## Deployment

### Docker Compose

**Standard Mode** (Proxies multiple domains via full URL)
```yaml
services:
  mediaproxy:
    image: ghcr.io/skidoodle/mediaproxy:latest
    container_name: mediaproxy
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - ALLOWED_DOMAINS=images.pexels.com,media.giphy.com
```

**Middleware Mode** (Transparent proxy for a single domain)
```yaml
services:
  mediaproxy:
    image: ghcr.io/skidoodle/mediaproxy:latest
    container_name: mediaproxy
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - BASE_URL=cdn.example.com
```

### Manual Installation

Requires Go 1.26 or higher and `libvips`.

```bash
go build -o mediaproxy .
./mediaproxy
```

## Usage

### Standard Mode
Default behavior. Pass the full media URL in the path.
`https://proxy.example.com/https://example.net/image.jpg`

### Middleware Mode
Activated by setting the `BASE_URL` environment variable. Acts as a transparent proxy for a single domain.
`https://proxy.example.com/image.jpg` → `https://BASE_URL/image.jpg`

## Configuration

| Variable | Description | Default |
| :--- | :--- | :--- |
| `LOG_LEVEL` | The logging level (`DEBUG`, `INFO`, `WARN`, `ERROR`). | `INFO` |
| `BASE_URL` | If set, activates middleware mode. Prepends `https://<BASE_URL>/` to paths. | (empty) |
| `CACHE_TTL` | The duration for which images are cached. | `10m` |
| `ALLOWED_DOMAINS` | Comma-separated list of domains to whitelist. | (empty) |
| `MAX_ALLOWED_SIZE` | Maximum media file size in bytes. | `52428800` |
| `DEFAULT_IMAGE_QUALITY` | Quality for optimized WebP images (1-100). | `80` |
| `CLIENT_TIMEOUT` | Timeout for fetching media from the origin. | `2m` |
| `ALLOW_PRIVATE_IPS` | Allow fetching from private/local IP ranges. | `false` |

## License

This project is licensed under the [GNU GPL v3](license).
