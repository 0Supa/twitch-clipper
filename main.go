package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

var httpAddr = "localhost:8989"

func resError(w http.ResponseWriter, message string, statusCode int) {
	m := map[string]interface{}{
		"message": message,
		"error":   statusCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(m)
}

func main() {
	http.HandleFunc("/clip/", func(w http.ResponseWriter, r *http.Request) {
		channelName := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/clip/"))
		if channelName == "" {
			resError(w, "invalid channel name", 400)
			return
		}

		path, err := MakeClip(channelName)
		if err != nil {
			resError(w, err.Error(), 500)
			return
		}

		m := map[string]interface{}{
			"path": path,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})

	log.Println("Server running on " + httpAddr)

	log.Fatal(http.ListenAndServe(httpAddr, nil))
}
