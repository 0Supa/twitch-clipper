package kick

import (
	"encoding/json"
	"fmt"
	"time"
	"twitch-clipper/clipper"

	"github.com/RomainMichau/cloudscraper_go/cloudscraper"
)

type channel struct {
	ID     int    `json:"id"`
	UserID int    `json:"user_id"`
	Slug   string `json:"slug"`
	User   struct {
		Username string `json:"username"`
	} `json:"user"`
	PlaybackURL string `json:"playback_url,omitempty"`
	Livestream  struct {
		ID         int        `json:"id"`
		Slug       string     `json:"slug"`
		CreatedAt  string     `json:"created_at"`
		Title      string     `json:"session_title"`
		IsLive     bool       `json:"is_live"`
		IsMature   bool       `json:"is_mature"`
		Language   string     `json:"language"`
		Categories []category `json:"categories"`
	} `json:"livestream"`
}

type category struct {
	ID         int    `json:"id"`
	CategoryID int    `json:"category_id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Category   struct {
		ID   int    `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"category"`
}

type ClipInfo struct {
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"createdAt"`
	Channel   channel   `json:"channel"`
}

var client *cloudscraper.CloudScrapper

func init() {
	client, _ = cloudscraper.Init(false, false)
}

func fetchChannel(channelName string) (response channel, err error) {
	res, err := client.Get("https://kick.com/api/v2/channels/"+channelName, make(map[string]string), "")
	if err != nil {
		return
	}

	if res.Status/100 != 2 {
		if res.Status == 404 {
			err = clipper.ErrStreamNotFound
		} else {
			err = fmt.Errorf("bad status code from kick (%v)", res.Status)
		}
		return
	}

	err = json.Unmarshal([]byte(res.Body), &response)
	if err != nil {
		return
	}

	return
}

func GetClipInfo(createdAt time.Time, channelName string) (info ClipInfo, err error) {
	res, err := fetchChannel(channelName)
	if err != nil {
		return
	}

	info = ClipInfo{
		Platform:  "kick",
		CreatedAt: createdAt.Truncate(time.Second),
		Channel:   res,
	}
	return
}
