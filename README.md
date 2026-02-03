# IPTV Manager

A web-based IPTV channel management tool for xTeVe and Emby.

## Features

- Search IPTV channels by playlist
- Preview channel streams in browser (HLS.js)
- Enable/disable channels per playlist
- Configure xTeVe channel mappings with nearby channel visibility
- Playlist-to-group-title auto-mapping
- Trigger Emby guide refresh
- File-based configuration (no database)

## Quick Start

### Docker (Recommended)

```bash
docker-compose up -d
```

Access at http://localhost:8080

### Manual

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Copy to embed location
cp -r web/dist cmd/server/web/dist/

# Build and run
go build -o iptv-manager ./cmd/server
./iptv-manager
```

## Configuration

Settings can be configured via the web UI Settings page, or by editing `config.json`:

```json
{
  "iptv": {
    "apiAddress": "https://your-provider/stalker_portal/server/load.php",
    "uid": "your-uid",
    "pass": "your-password"
  },
  "xteve": {
    "websocketAddress": "ws://localhost:34400/data/"
  },
  "emby": {
    "apiAddress": "http://localhost:8096",
    "apiKey": "your-api-key"
  },
  "playlistMappings": {
    "NO_EPG": "NO_EPG"
  }
}
```

## Environment Variables

- `DATA_DIR` - Directory for config.json (default: current directory)
- `--port` - Server port (default: 8080)
- `--config` - Path to config file
