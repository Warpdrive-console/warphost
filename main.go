package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/bluetooth/scan", bluetoothScan)
	r.GET("/bluetooth/devices", bluetoothDevices)
	r.Run()
}
