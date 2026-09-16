// Command preview serves the frontend with a mock backend, so the UI can be
// developed and screenshotted in a regular browser. It listens on loopback
// only and is not part of the application binary.
//
//	go run ./tools/preview
//	open http://127.0.0.1:34115/?lang=ru&theme=light&mode=vpn
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := flag.String("root", "frontend", "frontend directory containing dist/ and dev/")
	addr := flag.String("addr", "127.0.0.1:34115", "loopback address to listen on")
	flag.Parse()

	dist := filepath.Join(*root, "dist")
	mock := filepath.Join(*root, "dev", "mock.js")

	mux := http.NewServeMux()
	mux.HandleFunc("/wails/runtime.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		http.ServeFile(w, r, mock)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.FileServer(http.Dir(dist)).ServeHTTP(w, r)
			return
		}
		page, err := os.ReadFile(filepath.Join(dist, "index.html"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Wails injects its runtime into <head>; the preview injects the mock the same way.
		html := strings.Replace(string(page), "<head>", `<head><script src="/wails/runtime.js"></script>`, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	})

	if !strings.HasPrefix(*addr, "127.0.0.1:") && !strings.HasPrefix(*addr, "[::1]:") {
		log.Fatal("preview must listen on loopback")
	}
	log.Printf("preview on http://%s/", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
