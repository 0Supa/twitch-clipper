package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

var basePath = "/var/www/fi.supa.sh/clips"

type pCache struct {
	Expiry time.Time
	Body   string
}

var ErrStreamNotFound = errors.New("stream not found")

var playlistCache = map[string]pCache{}

var urlExp = regexp.MustCompile("https?://.+")
var m3SegmentExp = regexp.MustCompile("#EXTINF:.*live\n.+")

func FetchTwitchStream(channelName string, retries int) ([]string, error) {
	if retries > 3 {
		return nil, fmt.Errorf("failed fetching stream segments after %v tries", retries)
	}

	d := playlistCache[channelName]

	if time.Now().After(d.Expiry) {
		res, err := http.Get(
			fmt.Sprintf("https://luminous.alienpls.org/live/%s?platform=web&allow_source=true&allow_audio_only=true", url.PathEscape(channelName)),
		)
		if err != nil {
			return nil, err
		}

		defer res.Body.Close()
		buf, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		d.Body = string(buf)
		if res.StatusCode == http.StatusNotFound {
			return nil, ErrStreamNotFound
		} else if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("proxy -> bad status code (%v):\n%s", res.StatusCode, d.Body)
		}
	}

	streams := urlExp.FindAllString(d.Body, 1)
	if len(streams) == 0 {
		return nil, errors.New("no stream playlist available")
	}

	res, err := http.Get(streams[0])
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		d.Expiry = time.Now()
		playlistCache[channelName] = d
		return FetchTwitchStream(channelName, retries+1)
	}

	defer res.Body.Close()
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	filter := m3SegmentExp.FindAllString(string(buf), -1)
	if len(filter) == 0 {
		return FetchTwitchStream(channelName, retries+1)
	}

	segments := []string{}
	for _, s := range filter {
		segments = append(segments, s[strings.Index(s, "\n")+1:])
	}

	d.Expiry = time.Now().Add(time.Hour)

	playlistCache[channelName] = d

	return segments, nil
}

func MakeClip(channelName string) (string, error) {
	segments, err := FetchTwitchStream(channelName, 1)
	if err != nil {
		return "", err
	}

	segmentCount := len(segments)

	format := "mp4"
	clipID := time.Now().Unix()
	clipPath := fmt.Sprintf("%s/%s/%v.%s", basePath, channelName, clipID, format)

	buffer := make([][]byte, segmentCount)
	var wg sync.WaitGroup
	wg.Add(segmentCount)

	ch := make(chan error, segmentCount)
	for i, url := range segments {
		go func(i int, url string) {
			defer wg.Done()

			res, err := http.Get(url)
			if err != nil {
				ch <- err
				return
			}

			defer res.Body.Close()
			buf, err := io.ReadAll(res.Body)
			if err != nil {
				ch <- err
				return
			}
			buffer[i] = buf
		}(i, url)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for err := range ch {
		if err != nil {
			return "", err
		}
	}

	os.MkdirAll(path.Dir(clipPath), os.ModePerm)

	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-f", "mpegts",
		"-i", "-",
		"-c:v", "copy", "-c:a", "copy", "-c:s", "copy",
		"-f", format, clipPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		for _, d := range buffer {
			stdin.Write(d)
		}
		stdin.Close()
	}()

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%v.%s", channelName, clipID, format), err
}
