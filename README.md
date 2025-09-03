# warphost

Super simple local host based bluetooth and network manager written in go

Existing endpoints are

```js
GET /bluetooth/devices

GET /bluetooth/scan/pause
GET /bluetooth/scan/resume
GET /bluetooth/scan/status
```

This is hosted on 127.0.0.1:8080

To build, install go and run

```
go build
./warphost
```

this should run the server and everything will be awesome :sunglasses: