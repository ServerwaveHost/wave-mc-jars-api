# Wave MC Jars API

A unified REST API for downloading Minecraft server JARs from multiple official sources.

## Features

- **Unified API**: Single API to access multiple Minecraft server software
- **Proxy Downloads**: Downloads streamed through our API (no storage, no upstream URLs exposed)
- **Latest Build Support**: Use `/latest` to always get the most recent build
- **Java Version Info**: Automatic Java version requirements for each build
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

## Configuration

Create a `.env` file or set environment variables:

```bash
# Server
PORT=8080
GIN_MODE=release

# Redis (optional - falls back to memory cache)
# Format: redis://[[user]:password@]host:port[/db]
REDIS_URL=

# Cache TTL in seconds (default: 600 = 10 minutes)
CACHE_TTL=600

# Java version mapping config file path (default: java.json)
JAVA_CONFIG_PATH=java.json
```

### Java Version Mapping

Edit `java.json` to configure Java version requirements:

```json
{
  "servers": [
    { "min_version": "1.21", "java": 21 },
    { "min_version": "1.20.5", "java": 21 },
    { "min_version": "1.18", "java": 17 },
    { "min_version": "1.17", "java": 16 },
    { "min_version": "1.12", "java": 8 },
    { "min_version": "0", "java": 8 }
  ],
  "proxies": [
    { "min_version": "3.3", "java": 17 },
    { "min_version": "3.0", "java": 11 },
    { "min_version": "0", "java": 11 }
  ],
  "default": 17
}
```

## API Reference

### Quick Examples

```bash
# Download latest Paper 1.21.1
curl -OJ http://localhost:8080/categories/paper/versions/1.21.1/builds/latest/download

# Get build info with Java requirements
curl http://localhost:8080/categories/paper/versions/1.21.1/builds/latest

# Filter versions by type and date
curl "http://localhost:8080/categories/vanilla/versions?type=release&after=2024-01-01"
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

**Response includes Java version:**
```json
{
  "success": true,
  "data": {
    "number": 123,
    "version": "1.21.1",
    "stable": true,
    "downloads": [
      {
        "name": "paper-1.21.1-123.jar",
        "sha256": "abc123..."
      }
    ],
    "java": 21
  }
}
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
├── java.json              # Java version mapping config
├── .env.example
├── internal/
│   ├── cache/
│   │   └── cache.go
│   ├── handlers/
│   │   └── handlers.go
│   ├── java/
│   │   └── java.go
│   ├── models/
│   │   └── models.go
│   ├── providers/
│   │   ├── provider.go
│   │   ├── registry.go
│   │   ├── vanilla.go
│   │   ├── paper.go
│   │   ├── purpur.go
│   │   └── bungeecord.go
│   └── service/
│       └── service.go
├── go.mod
└── Dockerfile
```

## License

MIT License
