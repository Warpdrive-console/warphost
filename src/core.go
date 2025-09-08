package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type Game struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

var (
	gamesDir  = "./games"
	games     = make(map[string]Game)
	gamesLock sync.RWMutex
)

// Initialize core functionality
func initializeCore() error {
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		return fmt.Errorf("failed to create games directory: %v", err)
	}
	loadExistingGames()
	return nil
}

// CORS middleware adapted for Gin
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// Game headers middleware adapted for Gin
func addGameHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasSuffix(path, ".wasm") {
			c.Header("Content-Type", "application/wasm")
		} else if strings.HasSuffix(path, ".data") {
			c.Header("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(path, ".js") {
			c.Header("Content-Type", "application/javascript")
		} else if strings.HasSuffix(path, ".unityweb") {
			c.Header("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(path, ".pck") {
			c.Header("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(path, ".bundle") {
			c.Header("Content-Type", "application/octet-stream")
		}

		c.Header("Cross-Origin-Embedder-Policy", "require-corp")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")

		c.Next()
	}
}

func loadExistingGames() {
	entries, err := os.ReadDir(gamesDir)
	if err != nil {
		log.Printf("Warning: Could not read games directory: %v", err)
		return
	}

	gamesLock.Lock()
	defer gamesLock.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			id := entry.Name()
			indexPath := filepath.Join(gamesDir, id, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				games[id] = Game{
					ID:  id,
					URL: "/games/" + id + "/index.html",
				}
				log.Printf("Loaded existing game: %s", id)
			}
		}
	}
}

func handleUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .zip files allowed"})
		return
	}

	id := generateSafeID(header.Filename)
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}

	tmpZip := filepath.Join(os.TempDir(), fmt.Sprintf("game_%s.zip", id))
	out, err := os.Create(tmpZip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save temp zip"})
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tmpZip)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file copy error: " + err.Error()})
		return
	}
	out.Close()
	defer os.Remove(tmpZip)

	dest := filepath.Join(gamesDir, id)
	os.RemoveAll(dest)
	if err := unzip(tmpZip, dest); err != nil {
		os.RemoveAll(dest)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot unzip: " + err.Error()})
		return
	}

	indexPath := filepath.Join(dest, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		os.RemoveAll(dest)
		c.JSON(http.StatusBadRequest, gin.H{"error": "no index.html found in zip"})
		return
	}

	url := "/games/" + id + "/index.html"
	game := Game{ID: id, URL: url}

	gamesLock.Lock()
	games[id] = game
	gamesLock.Unlock()

	c.JSON(http.StatusOK, game)
	log.Printf("Uploaded game: %s", id)
}

func generateSafeID(filename string) string {
	id := strings.TrimSuffix(filename, filepath.Ext(filename))
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "-", "_")

	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	cleaned := result.String()
	if len(cleaned) == 0 {
		return ""
	}
	if cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "game_" + cleaned
	}
	return cleaned
}

func handleList(c *gin.Context) {
	gamesLock.RLock()
	defer gamesLock.RUnlock()

	list := make([]Game, 0, len(games))
	for _, g := range games {
		list = append(list, g)
	}

	c.JSON(http.StatusOK, list)
}

func handleOpen(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	gamesLock.RLock()
	game, ok := games[id]
	gamesLock.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}

	c.JSON(http.StatusOK, game)
}

func handleDelete(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	gamesLock.Lock()
	game, ok := games[id]
	if ok {
		delete(games, id)
	}
	gamesLock.Unlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}

	dest := filepath.Join(gamesDir, id)
	if err := os.RemoveAll(dest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete game files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "deleted",
		"id":     game.ID,
	})
	log.Printf("Deleted game: %s", id)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		fpath := filepath.Join(dest, cleanName)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}
		if strings.Contains(cleanName, "..") {
			log.Printf("Skipping suspicious file: %s", f.Name)
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		if err := extractFile(f, fpath); err != nil {
			return fmt.Errorf("failed to extract %s: %v", f.Name, err)
		}
	}
	return nil
}

func extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return err
	}

	return nil
}
