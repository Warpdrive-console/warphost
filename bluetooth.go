package main

import (
	"fmt"
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"io"

	"github.com/gin-gonic/gin"
)

type DeviceCache struct {
	sync.RWMutex
	devices map[string]string
}

type BluetoothScanner struct {
	cache     *DeviceCache
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	pauseChan chan bool
	isPaused  bool
	mutex     sync.RWMutex
}

var cache = &DeviceCache{devices: make(map[string]string)}
var scanner *BluetoothScanner

func NewBluetoothScanner() *BluetoothScanner {
	return &BluetoothScanner{
		cache:     cache,
		pauseChan: make(chan bool, 1),
		isPaused:  false,
	}
}

func (bs *BluetoothScanner) Start(ctx context.Context) error {
	bs.cmd = exec.Command("bluetoothctl")
	var err error
	bs.stdin, err = bs.cmd.StdinPipe()
	if err != nil {
		return err
	}
	
	stdout, err := bs.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	
	stderr, err := bs.cmd.StderrPipe()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(io.MultiReader(stdout, stderr))
	
	err = bs.cmd.Start()
	if err != nil {
		return err
	}
	
	// Initialize bluetooth
	bs.stdin.Write([]byte("power on\nagent on\ndefault-agent\nscan on\n"))

	go func() {
		defer bs.cmd.Wait()
		re := regexp.MustCompile(`Device\s+([0-9A-Fa-f:]{17})\s+(.+)$`)
		cmdre := regexp.MustCompile(`(NEW|CHG|DEL)`)
		
		for {
			select {
			case <-ctx.Done():
				return
			case paused := <-bs.pauseChan:
				bs.mutex.Lock()
				bs.isPaused = paused
				if paused {
					fmt.Println("Pausing bluetooth scan...")
					bs.stdin.Write([]byte("scan off\n"))
				} else {
					fmt.Println("Resuming bluetooth scan...")
					bs.stdin.Write([]byte("scan on\n"))
				}
				bs.mutex.Unlock()
				continue
			default:
			}
			
			// Only process if not paused
			bs.mutex.RLock()
			if bs.isPaused {
				bs.mutex.RUnlock()
				continue
			}
			bs.mutex.RUnlock()
			
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			
			line = strings.TrimSpace(line)
			
			// Check if line has enough parts before splitting
			parts := strings.Split(line, " ")
			if len(parts) < 2 {
				continue
			}
			
			cmd := parts[1]
			s := cmdre.FindStringSubmatch(cmd)
			
			if s != nil && len(s) > 0 && s[0] == "NEW" {
				m := re.FindStringSubmatch(line)
				if m != nil && len(m) > 2 {
					fmt.Println("got new device")
					mac := strings.ToUpper(m[1])
					name := strings.TrimSpace(m[2])
					bs.cache.Lock()
					bs.cache.devices[mac] = name
					bs.cache.Unlock()
				}
			}
		}
	}()

	return nil
}

func (bs *BluetoothScanner) Pause() {
	select {
	case bs.pauseChan <- true:
	default:
	}
}

func (bs *BluetoothScanner) Resume() {
	select {
	case bs.pauseChan <- false:
	default:
	}
}

func (bs *BluetoothScanner) IsPaused() bool {
	bs.mutex.RLock()
	defer bs.mutex.RUnlock()
	return bs.isPaused
}

func (bs *BluetoothScanner) Stop() {
	if bs.stdin != nil {
		bs.stdin.Write([]byte("scan off\nquit\n"))
		bs.stdin.Close()
	}
	if bs.cmd != nil {
		bs.cmd.Process.Kill()
	}
}

func startBluetoothScanner(ctx context.Context) error {
	scanner = NewBluetoothScanner()
	return scanner.Start(ctx)
}

func bluetoothDevices(c *gin.Context) {
	cache.RLock()
	defer cache.RUnlock()

	macList := make([]string, 0, len(cache.devices))
	nameList := make([]string, 0, len(cache.devices))
	devices := make([]gin.H, 0, len(cache.devices))
	for mac, name := range cache.devices {
		macList = append(macList, mac)
		nameList = append(nameList, name)
		devices = append(devices, gin.H{"mac": mac, "name": name})
	}

	c.JSON(200, gin.H{
		"count":     len(cache.devices),
		"deviceMap": cache.devices,
		"macList":   macList,
		"nameList":  nameList,
		"devices":   devices,
		"paused":    scanner.IsPaused(),
	})
}

func bluetoothPause(c *gin.Context) {
	if scanner != nil {
		scanner.Pause()
		c.JSON(200, gin.H{"status": "paused"})
	} else {
		c.JSON(400, gin.H{"error": "scanner not initialized"})
	}
}

func bluetoothResume(c *gin.Context) {
	if scanner != nil {
		scanner.Resume()
		c.JSON(200, gin.H{"status": "resumed"})
	} else {
		c.JSON(400, gin.H{"error": "scanner not initialized"})
	}
}

func bluetoothStatus(c *gin.Context) {
	if scanner != nil {
		c.JSON(200, gin.H{
			"paused": scanner.IsPaused(),
			"status": func() string {
				if scanner.IsPaused() {
					return "paused"
				}
				return "scanning"
			}(),
		})
	} else {
		c.JSON(400, gin.H{"error": "scanner not initialized"})
	}
}