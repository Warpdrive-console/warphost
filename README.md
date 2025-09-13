# warphost

Super simple local host based bluetooth and network manager written in go

Existing endpoints are

# Bluetooth
```js
GET /bluetooth/scan
GET /bluetooth/devices

GET /bluetooth/scan/pause
GET /bluetooth/scan/resume
GET /bluetooth/scan/status

GET /bluetooth/connect/:mac
GET /bluetooth/disconnect/:mac
GET /bluetooth/forget/:mac
```

# Core (Game/Web App Management)
```js
POST /core/upload        // Upload ZIP file containing web app/game
GET  /core/list          // List all uploaded games/apps
GET  /core/open?id=<id>  // Get game/app details by ID
DELETE /core/delete?id=<id> // Delete game/app by ID

Static file serving: /games/<id>/ // Serve game files with proper CORS/MIME headers
```

This is hosted on 127.0.0.1:8080

To build, install go and do

```
cd src
go build
./warphost
```

this should run the server and everything will be awesome :sunglasses:

## Core API Usage

The core API allows you to upload, manage, and serve web applications/games:

### Upload a Game/Web App

```bash
curl -X POST -F "file=@mygame.zip" http://127.0.0.1:8080/core/upload
```

### List All Games

```bash
curl http://127.0.0.1:8080/core/list
```

### Open a Specific Game

```bash
curl "http://127.0.0.1:8080/core/open?id=mygame"
```

### Delete a Game

```bash
curl -X DELETE "http://127.0.0.1:8080/core/delete?id=mygame"
```

### Access Game Files

After uploading, games are accessible at: `http://127.0.0.1:8080/games/<game-id>/index.html`

**Note**: Uploaded ZIP files must contain an `index.html` file in the root to be valid.
