package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	if err := startBluetoothScanner(ctx); err != nil {
		panic("failed to start scanner: " + err.Error())
	}

	// Initialize core functionality
	if err := initializeCore(); err != nil {
		log.Fatalf("Failed to initialize core: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	r.Use(corsMiddleware())

	r.GET("/bluetooth/scan", bluetoothScan)
	r.GET("/bluetooth/devices", bluetoothDevices)

	r.GET("/bluetooth/connect/:mac", bluetoothConnect)
	r.GET("/bluetooth/disconnect/:mac", bluetoothDisconnect)
	r.GET("/bluetooth/forget/:mac", bluetoothForget)

	r.GET("/bluetooth/scan/resume", bluetoothResume)
	r.GET("/bluetooth/scan/pause", bluetoothPause)
	r.GET("/bluetooth/scan/status", bluetoothStatus)

	r.POST("/core/upload", handleUpload)
	r.GET("/core/list", handleList)
	r.GET("/core/open", handleOpen)
	r.DELETE("/core/delete", handleDelete)

	gameRouter := r.Group("/games")
	gameRouter.Use(addGameHeaders())
	gameRouter.Static("/", gamesDir)

	r.Run(":8080")
}
