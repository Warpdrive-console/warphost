package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func main() {
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		log.Fatalf("Failed to create games directory: %v", err)
	}

	loadExistingGames()

	http.HandleFunc("/upload", corsMiddleware(handleUpload))
	http.HandleFunc("/list", corsMiddleware(handleList))
	http.HandleFunc("/open", corsMiddleware(handleOpen))
	http.HandleFunc("/delete", corsMiddleware(handleDelete))

	http.Handle("/games/",
		corsMiddleware(addGameHeaders(
			http.StripPrefix("/games/", http.FileServer(http.Dir(gamesDir))))))

	fmt.Println("Warpdrive Webserver running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Extra MIME support for Unity/Godot/GMS2
func addGameHeaders(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		} else if strings.HasSuffix(r.URL.Path, ".data") {
			w.Header().Set("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(r.URL.Path, ".unityweb") {
			w.Header().Set("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(r.URL.Path, ".pck") {
			w.Header().Set("Content-Type", "application/octet-stream")
		} else if strings.HasSuffix(r.URL.Path, ".bundle") {
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")

		next.ServeHTTP(w, r)
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

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(0); err != nil {
		http.Error(w, "parse form failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		http.Error(w, "only .zip files allowed", http.StatusBadRequest)
		return
	}

	id := generateSafeID(header.Filename)
	if id == "" {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	tmpZip := filepath.Join(os.TempDir(), fmt.Sprintf("game_%s.zip", id))
	out, err := os.Create(tmpZip)
	if err != nil {
		http.Error(w, "cannot save temp zip", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tmpZip)
		http.Error(w, "file copy error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out.Close()
	defer os.Remove(tmpZip)

	dest := filepath.Join(gamesDir, id)
	os.RemoveAll(dest)
	if err := unzip(tmpZip, dest); err != nil {
		os.RemoveAll(dest)
		http.Error(w, "cannot unzip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	indexPath := filepath.Join(dest, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		os.RemoveAll(dest)
		http.Error(w, "no index.html found in zip", http.StatusBadRequest)
		return
	}

	url := "/games/" + id + "/index.html"
	game := Game{ID: id, URL: url}

	gamesLock.Lock()
	games[id] = game
	gamesLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
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

func handleList(w http.ResponseWriter, r *http.Request) {
	gamesLock.RLock()
	defer gamesLock.RUnlock()

	list := make([]Game, 0, len(games))
	for _, g := range games {
		list = append(list, g)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func handleOpen(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	gamesLock.RLock()
	game, ok := games[id]
	gamesLock.RUnlock()

	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "DELETE only", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	gamesLock.Lock()
	game, ok := games[id]
	if ok {
		delete(games, id)
	}
	gamesLock.Unlock()

	if !ok {
		http.Error(w, "game not found", http.StatusNotFound)
		return
	}

	dest := filepath.Join(gamesDir, id)
	if err := os.RemoveAll(dest); err != nil {
		http.Error(w, "failed to delete game files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
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
