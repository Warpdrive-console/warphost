package main

import (
	"context"
	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	if err := startBluetoothScanner(ctx); err != nil {
		panic("failed to start scanner: " + err.Error())
	}

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.GET("/bluetooth/scan", bluetoothScan)
	r.GET("/bluetooth/devices", bluetoothDevices)

	r.GET("/bluetooth/connect/:mac", bluetoothConnect)
	r.GET("/bluetooth/disconnect/:mac", bluetoothConnect)

	r.GET("/bluetooth/scan/resume", bluetoothResume)
	r.GET("/bluetooth/scan/pause", bluetoothPause)
	r.GET("/bluetooth/scan/status", bluetoothStatus)
	r.Run()
}
