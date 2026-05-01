package ui

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
)

//go:embed static/* static/assets/*
var assets embed.FS

func Handler() (http.Handler, error) {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			indexFile, err := staticFS.Open("index.html")
			if err != nil {
				http.Error(w, "ui index not found", http.StatusInternalServerError)
				return
			}
			defer func() {
				_ = indexFile.Close()
			}()

			indexBytes, err := io.ReadAll(indexFile)
			if err != nil {
				http.Error(w, "failed to read ui index", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexBytes)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
