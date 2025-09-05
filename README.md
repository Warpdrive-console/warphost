# warphost

Super simple local host based bluetooth and network manager written in go

Existing endpoints are

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

This is hosted on 127.0.0.1:8080

To build, install go and do

```
cd src
go build
./warphost
```

this should run the server and everything will be awesome :sunglasses: