# Wave MC Jars API

A unified REST API for downloading Minecraft server JARs from multiple official sources.

## Features

- **Unified API**: Single API to access multiple Minecraft server software
- **Proxy Downloads**: Downloads streamed through our API (no storage, no upstream URLs exposed)
- **Latest Build Support**: Use `/latest` to always get the most recent build
- **Filtering**: Filter versions by date, type (release/snapshot), and stability
- **Redis Caching**: Optional Redis support with configurable TTL (falls back to memory cache)
- **Official Sources Only**: Always fetches from official APIs

## Supported Categories

### Game Servers

| Category | Name | Source |
|----------|------|--------|
| `vanilla` | Vanilla | Mojang |
| `paper` | Paper | PaperMC |
| `folia` | Folia | PaperMC |
| `purpur` | Purpur | PurpurMC |

### Proxy Servers

| Category | Name | Source |
|----------|------|--------|
| `velocity` | Velocity | PaperMC |
| `waterfall` | Waterfall | PaperMC |
| `bungeecord` | BungeeCord | SpigotMC Jenkins |

### Not Supported

- **Spigot/Bukkit**: Due to DMCA restrictions, Spigot cannot be distributed directly. Users must compile it using [BuildTools](https://www.spigotmc.org/wiki/buildtools/).

## Quick Start

### Running Locally

```bash
git clone https://github.com/serverwave/wave-mc-jars-api.git
cd wave-mc-jars-api

# Copy and configure environment
cp .env.example .env

# Install dependencies
go mod tidy

# Run the server
go run main.go
```

### Using Docker

```bash
docker build -t wave-mc-jars-api .
docker run -p 8080:8080 wave-mc-jars-api
```

### With Redis

```bash
# Start Redis
docker run -d -p 6379:6379 redis

# Configure .env
REDIS_URL=redis://localhost:6379
CACHE_TTL=600

# Run the API
go run main.go
```

## Configuration

Create a `.env` file or set environment variables:

```bash
# Server
PORT=8080
GIN_MODE=release

# Redis (optional - falls back to memory cache)
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Cache TTL in seconds (default: 600 = 10 minutes)
CACHE_TTL=600
```

## API Reference

### Quick Examples

```bash
# Download latest Paper 1.21.1
curl -OJ http://localhost:8080/categories/paper/versions/1.21.1/builds/latest/download

# Download latest Vanilla
curl -OJ http://localhost:8080/categories/vanilla/versions/1.21.1/builds/latest/download

# Download latest BungeeCord
curl -OJ http://localhost:8080/categories/bungeecord/versions/latest/builds/latest/download

# Get build info
curl http://localhost:8080/categories/purpur/versions/1.21.1/builds/latest

# Filter versions by type and date
curl "http://localhost:8080/categories/vanilla/versions?type=release&after=2024-01-01"

# Search with filters
curl "http://localhost:8080/search?q=1.21&category=paper&stable=true"
```

### Endpoints

#### List Categories

```http
GET /categories
```

#### Get Category

```http
GET /categories/{category}
```

#### List Versions

```http
GET /categories/{category}/versions
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `type` | string | Filter by type: `release`, `snapshot`, `beta`, `alpha` |
| `stable` | bool | Set to `true` for stable versions only |
| `after` | date | Versions released after this date (YYYY-MM-DD) |
| `before` | date | Versions released before this date (YYYY-MM-DD) |
| `min_year` | int | Minimum release year |
| `max_year` | int | Maximum release year |

Example:
```bash
# Get only release versions from 2024
curl "http://localhost:8080/categories/vanilla/versions?type=release&min_year=2024"

# Get snapshots between dates
curl "http://localhost:8080/categories/vanilla/versions?type=snapshot&after=2024-06-01&before=2024-12-31"
```

#### List Builds

```http
GET /categories/{category}/versions/{version}/builds
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `stable` | bool | Set to `true` for stable builds only |
| `after` | date | Builds created after this date (YYYY-MM-DD) |
| `before` | date | Builds created before this date (YYYY-MM-DD) |

#### Get Build

```http
GET /categories/{category}/versions/{version}/builds/{build}
```

Use `latest` to get the latest build:

```http
GET /categories/paper/versions/1.21.1/builds/latest
```

#### Download JAR

```http
GET /categories/{category}/versions/{version}/builds/{build}/download
```

#### Search

```http
GET /search
```

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Search query |
| `category` | string | Filter by category |
| `type` | string | Filter by version type |
| `stable` | bool | Stable versions only |
| `after` | date | Released after date |
| `before` | date | Released before date |
| `min_year` | int | Minimum release year |
| `max_year` | int | Maximum release year |

## Architecture

```
wave-mc-jars-api/
├── main.go
├── .env.example
├── internal/
│   ├── cache/
│   │   └── cache.go          # Redis + memory cache
│   ├── handlers/
│   │   └── handlers.go       # HTTP handlers
│   ├── models/
│   │   └── models.go         # Data models
│   ├── providers/
│   │   ├── provider.go       # Provider interface
│   │   ├── registry.go       # Provider registry
│   │   ├── vanilla.go        # Mojang/Vanilla
│   │   ├── paper.go          # Paper/Folia/Velocity/Waterfall
│   │   ├── purpur.go         # Purpur
│   │   └── bungeecord.go     # BungeeCord (Jenkins)
│   └── service/
│       └── service.go        # Business logic
├── go.mod
└── Dockerfile
```

## How It Works

1. Client requests download from our API
2. API resolves the build (including `latest`)
3. API streams the JAR directly from upstream to client
4. **No file is stored on our server**
5. **Upstream URLs are never exposed to clients**

## License

MIT License
