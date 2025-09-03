package main

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func performBluetoothScan(timeoutSec float64) (map[string]string, []string, []string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec*float64(time.Second))+2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bluetoothctl")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}

	// Prime controller
	initCmds := []string{
		"power on",
		"agent on",
		"default-agent",
		"scan on",
	}
	go func() {
		for _, c := range initCmds {
			_, _ = stdin.Write([]byte(c + "\n"))
			time.Sleep(40 * time.Millisecond)
		}
	}()

	re := regexp.MustCompile(`^Device\s+([0-9A-Fa-f:]{17})\s+(.+)$`)
	deviceMap := make(map[string]string)

	scanner := bufio.NewScanner(stdout)
	end := time.Now().Add(time.Duration(timeoutSec * float64(time.Second)))

	for {
		if time.Now().After(end) {
			break
		}
		if !scanner.Scan() {
			// slight pause to allow more data unless context done
			select {
			case <-ctx.Done():
				break
			default:
				time.Sleep(15 * time.Millisecond)
				continue
			}
		}
		line := strings.TrimSpace(scanner.Text())
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			mac := m[1]
			name := strings.TrimSpace(m[2])
			if _, exists := deviceMap[mac]; !exists {
				deviceMap[mac] = name
			}
		}
	}

	// Stop scan and exit cleanly
	_, _ = stdin.Write([]byte("scan off\n"))
	time.Sleep(120 * time.Millisecond)
	_, _ = stdin.Write([]byte("exit\n"))
	stdin.Close()
	_ = cmd.Wait()

	macList := make([]string, 0, len(deviceMap))
	nameList := make([]string, 0, len(deviceMap))
	for mac, name := range deviceMap {
		macList = append(macList, mac)
		nameList = append(nameList, name)
	}

	return deviceMap, macList, nameList, nil
}

func bluetoothScan(c *gin.Context) {
	timeoutStr := c.DefaultQuery("timeout", "5")
	timeoutSec, err := strconv.ParseFloat(timeoutStr, 64)
	if err != nil || timeoutSec <= 0 {
		c.String(400, "Invalid timeout value")
		return
	}

	start := time.Now()
	deviceMap, macList, nameList, err := performBluetoothScan(timeoutSec)
	elapsed := time.Since(start)

	if err != nil {
		c.JSON(500, gin.H{"error": "Scan failed", "details": err.Error()})
		return
	}

	devices := make([]gin.H, 0, len(deviceMap))
	for i, mac := range macList {
		devices = append(devices, gin.H{"mac": mac, "name": nameList[i]})
	}

	c.JSON(200, gin.H{
		"timeoutSec": timeoutSec,
		"elapsedMs":  elapsed.Milliseconds(),
		"method":     "live",
		"count":      len(deviceMap),
		"deviceMap":  deviceMap,
		"macList":    macList,
		"nameList":   nameList,
		"devices":    devices,
	})
}

func bluetoothDevices(c *gin.Context) {
	out, err := exec.Command("bluetoothctl", "devices").Output()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list devices", "details": err.Error()})
		return
	}
	lines := strings.Split(string(out), "\n")
	re := regexp.MustCompile(`^Device\s+([0-9A-Fa-f:]{17})\s+(.+)$`)
	deviceMap := make(map[string]string)
	macList := []string{}
	nameList := []string{}
	for _, line := range lines {
		m := re.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) == 3 {
			mac := m[1]
			name := strings.TrimSpace(m[2])
			if _, exists := deviceMap[mac]; !exists {
				deviceMap[mac] = name
				macList = append(macList, mac)
				nameList = append(nameList, name)
			}
		}
	}
	devices := make([]gin.H, 0, len(macList))
	for i, mac := range macList {
		devices = append(devices, gin.H{"mac": mac, "name": nameList[i]})
	}
	c.JSON(200, gin.H{
		"count":     len(deviceMap),
		"deviceMap": deviceMap,
		"macList":   macList,
		"nameList":  nameList,
		"devices":   devices,
	})
}
