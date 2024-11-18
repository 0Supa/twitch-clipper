package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"twitch-clipper/utils"
)

const httpHost = "localhost:8989"
const clipsDir = "/var/www/fi.supa.sh/clips"

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

		saveDir := fmt.Sprintf("%s/%s", clipsDir, channelName)

		createdAt := time.Now().UTC()
		clipID := fmt.Sprintf("%v", createdAt.Unix())

		var futile bool

		query := r.URL.Query()
		go func() {
			infoPath := fmt.Sprintf("%s/%s.info.json", saveDir, clipID)

			clipInfo, err := utils.GetClipInfo(createdAt, channelName, query.Get("creator_id"), query.Get("parent_id"))
			if err != nil {
				log.Println("clip info failed", err)
				return
			}

			if clipInfo.Channel == nil {
				futile = true
				resError(w, "channel not found", 404)
				return
			}

			data, err := json.Marshal(clipInfo)
			if err != nil {
				log.Println("clip info marshal failed", err)
				return
			}

			os.MkdirAll(saveDir, os.ModePerm)
			os.WriteFile(infoPath, data, 0644)
		}()

		path, err := MakeClip(saveDir, clipID, channelName)
		if futile {
			return
		}
		if err != nil {
			statusCode := 500
			if err == ErrStreamNotFound {
				statusCode = 404
			}

			resError(w, err.Error(), statusCode)
			return
		}

		m := map[string]interface{}{
			"path": path,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})

	log.Println("Server running on " + httpHost)

	log.Fatal(http.ListenAndServe(httpHost, nil))
}
