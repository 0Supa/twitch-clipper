package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"twitch-clipper/clipper"
	"twitch-clipper/kick"
	"twitch-clipper/twitch"
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

		query := r.URL.Query()
		infoPath := fmt.Sprintf("%s/%s.info.json", saveDir, clipID)

		var playlistURL string
		var data []byte
		if query.Get("platform") == "kick" {
			clipInfo, err := kick.GetClipInfo(createdAt, channelName)
			if err != nil {
				statusCode := 500
				if err == clipper.ErrStreamNotFound {
					statusCode = 404
				}

				log.Println("kick clip info failed", err)
				resError(w, "failed to fetch clip info: "+err.Error(), statusCode)
				return
			}

			if clipInfo.Channel.ID == 0 {
				resError(w, "channel not found", 404)
				return
			}

			playlistURL = clipInfo.Channel.PlaybackURL

			log.Printf("clipped kick/%s\n", clipInfo.Channel.Slug)

			data, err = json.Marshal(clipInfo)
			if err != nil {
				log.Println("clip info marshal failed", err)
				return
			}
		} else {
			clipInfo, err := twitch.GetClipInfo(createdAt, channelName, query.Get("creator_id"), query.Get("parent_id"))
			if err != nil {
				log.Println("twitch clip info failed", err)
				resError(w, "failed to fetch clip info: "+err.Error(), 500)
				return
			}

			if clipInfo.Channel == nil {
				resError(w, "channel not found", 404)
				return
			}

			playlistURL = fmt.Sprintf("https://luminous.alienpls.org/live/%s?platform=web&allow_source=true&allow_audio_only=true", channelName)

			var clipLog strings.Builder
			if clipInfo.Creator != nil {
				clipLog.WriteString(fmt.Sprintf("@%s ", clipInfo.Creator.Login))
			}
			clipLog.WriteString(fmt.Sprintf("clipped twitch/%s", clipInfo.Channel.Login))
			if clipInfo.Parent != nil {
				clipLog.WriteString(fmt.Sprintf(" from #%s", clipInfo.Parent.Login))
			}
			log.Println(clipLog.String())

			data, err = json.Marshal(clipInfo)
			if err != nil {
				log.Println("clip info marshal failed", err)
				return
			}
		}

		os.MkdirAll(saveDir, os.ModePerm)
		os.WriteFile(infoPath, data, 0644)

		playlist, err := clipper.FetchPlaylist(playlistURL, 0)
		if err != nil {
			statusCode := 500
			if err == clipper.ErrStreamNotFound {
				statusCode = 404
			}

			resError(w, err.Error(), statusCode)
			return
		}

		path, err := clipper.MakeClip(saveDir, clipID, channelName, playlist)
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

	log.Println("Server running on " + httpHost)

	log.Fatal(http.ListenAndServe(httpHost, nil))
}
