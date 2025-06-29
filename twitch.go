package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
)

type pCache struct {
	Expiry         time.Time
	MasterPlaylist *m3u8.MasterPlaylist
}

var ErrStreamNotFound = errors.New("stream not found")

var playlistCache = map[string]pCache{}

var httpClient = &http.Client{Timeout: time.Minute}

func FetchTwitchStream(channelName string, retries int) (*m3u8.MediaPlaylist, error) {
	if retries > 3 {
		return nil, fmt.Errorf("failed fetching stream segments after %v tries", retries)
	}

	d := playlistCache[channelName]

	if time.Now().After(d.Expiry) {
		res, err := httpClient.Get(
			fmt.Sprintf("https://luminous.alienpls.org/live/%s?platform=web&allow_source=true&allow_audio_only=true", url.PathEscape(channelName)),
		)
		if err != nil {
			return nil, err
		}

		defer res.Body.Close()

		if res.StatusCode == http.StatusNotFound {
			return nil, ErrStreamNotFound
		} else if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("proxy -> bad status code (%v)", res.StatusCode)
		}

		p, _, err := m3u8.DecodeFrom(res.Body, false)
		if err != nil {
			return nil, err
		}

		d.MasterPlaylist = p.(*m3u8.MasterPlaylist)
	}

	variants := d.MasterPlaylist.Variants

	if len(variants) == 0 {
		return nil, errors.New("no stream playlist available")
	}

	SortVariantsByResolution(variants)

	res, err := httpClient.Get(variants[0].URI)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	mediaPlaylist, _, err := m3u8.DecodeFrom(res.Body, true)
	if err != nil {
		d.Expiry = time.Now()
		playlistCache[channelName] = d
		return FetchTwitchStream(channelName, retries+1)
	}

	d.Expiry = time.Now().Add(time.Hour)

	playlistCache[channelName] = d

	return mediaPlaylist.(*m3u8.MediaPlaylist), nil
}

func MakeClip(saveDir string, clipID string, channelName string) (string, error) {
	playlist, err := FetchTwitchStream(channelName, 1)
	if err != nil {
		return "", err
	}

	segmentCount := playlist.Count()

	format := "mp4"
	clipPath := fmt.Sprintf("%s/%s.%s", saveDir, clipID, format)

	buffer := make([][]byte, segmentCount)
	var wg sync.WaitGroup
	wg.Add(int(segmentCount))

	var futile bool
	ch := make(chan error, segmentCount)
	for i, seg := range playlist.Segments[:segmentCount] {
		go func(i int, url string) {
			defer wg.Done()

			res, err := httpClient.Get(url)
			if err != nil && !futile {
				ch <- err
				return
			}

			defer res.Body.Close()
			buf, err := io.ReadAll(res.Body)
			if !futile {
				if err != nil {
					ch <- err
					return
				}
				buffer[i] = buf
			}
		}(i, seg.URI)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for err := range ch {
		if err != nil {
			futile = true
			return "", err
		}
	}

	os.MkdirAll(saveDir, os.ModePerm)

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

	return fmt.Sprintf("%s/%s.%s", channelName, clipID, format), err
}

func SortVariantsByResolution(variants []*m3u8.Variant) {
	parseRes := func(res string) (w, h int) {
		parts := strings.Split(res, "x")
		if len(parts) != 2 {
			return 0, 0
		}
		w, _ = strconv.Atoi(parts[0])
		h, _ = strconv.Atoi(parts[1])
		return
	}
	sort.SliceStable(variants, func(i, j int) bool {
		w1, h1 := parseRes(variants[i].Resolution)
		w2, h2 := parseRes(variants[j].Resolution)
		if w1 != w2 {
			return w1 > w2
		}
		return h1 > h2
	})
}
