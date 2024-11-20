package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const twitchClientID = "ue6666qo983tsx6so1t0vnawi233wa"

type twitchGQLPayload struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variables     any    `json:"variables"`
}

type userData struct {
	ID                string `json:"id"`
	Login             string `json:"login"`
	DisplayName       string `json:"displayName"`
	BroadcastSettings *struct {
		Game struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"game"`
		Title string `json:"title"`
	} `json:"broadcastSettings,omitempty"`
	Stream *struct {
		CreatedAt string `json:"createdAt"`
		IsMature  bool   `json:"isMature"`
		Language  string `json:"language"`
	} `json:"stream,omitempty"`
}

type twitchUsersResponse struct {
	Data struct {
		Channel *userData   `json:"channel"`
		Users   []*userData `json:"users"`
	} `json:"data"`
}

type ClipInfo struct {
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"createdAt"`
	Channel   *userData `json:"channel"`
	Creator   *userData `json:"creator"`
	Parent    *userData `json:"parent"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func fetchClipUsers(channelName string, userIDs ...string) (response twitchUsersResponse, err error) {
	payload, err := json.Marshal(twitchGQLPayload{
		OperationName: "Users",
		Query:         "query Users($channel: String! $users: [ID!]!) { channel: user(login: $channel) { id login displayName broadcastSettings { game { id name } title } stream { createdAt isMature language } } users(ids: $users) { id login displayName } }",
		Variables: map[string]any{
			"channel": channelName,
			"users":   userIDs,
		},
	})
	if err != nil {
		return
	}

	req, _ := http.NewRequest("POST", "https://gql.twitch.tv/gql", bytes.NewBuffer(payload))
	req.Header.Set("Client-Id", twitchClientID)

	res, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return
	}

	return
}

func GetClipInfo(createdAt time.Time, channelName string, creatorID string, parentID string) (info ClipInfo, err error) {
	res, err := fetchClipUsers(channelName, creatorID, parentID)
	if err != nil {
		return
	}

	if len(res.Data.Users) < 2 {
		err = errors.New("upstream error")
		return
	}

	info = ClipInfo{
		Platform:  "twitch",
		CreatedAt: createdAt.Truncate(time.Second),
		Channel:   res.Data.Channel,
		Creator:   res.Data.Users[0],
		Parent:    res.Data.Users[1],
	}
	return
}
