package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	natpmp "github.com/jackpal/go-nat-pmp"
)

func main() {
	pmp := natpmp.NewClient(net.IPv4(10, 2, 0, 1))
	_, err := pmp.AddPortMapping("udp", 1, 0, 300)
	if err != nil {
		panic(err)
	}

	res, err := pmp.AddPortMapping("tcp", 1, 0, 300)
	if err != nil {
		panic(err)
	}

	jsonData := fmt.Sprintf("{\"listen_port\":%d}", res.MappedExternalPort)
	data := make(url.Values, 1)
	data.Add("json", jsonData)
	resp, err := http.PostForm("http://localhost:8080/api/v2/app/setPreferences", data)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	slog.Info("response status", "status", resp.Status, "port", res.MappedExternalPort)

	// get stalled download
	resp, err = http.Get("http://localhost:8080/api/v2/torrents/info?filter=downloading")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic(fmt.Errorf("status code %d", resp.StatusCode))
	}
	torrents, err := getDownloadLists()
	if err != nil {
		panic(err)
	}

	someRunning := len(torrents) == 0
	for _, torrent := range torrents {
		lastActivity := time.Unix(int64(torrent.LastActivity), 0)
		if time.Since(lastActivity) < 1*time.Minute {
			slog.Info("found new download", "name", torrent.Name, "last_activity", lastActivity)
			someRunning = true
		} else if torrent.State == "stalledDL" || torrent.State == "metaDL" {
			slog.Error("found stalled download", "name", torrent.Name, "progres", torrent.Progress)
			peers, err := getTorrentPeers(torrent.Hash)
			if err != nil {
				panic(err)
			}
			if peers > 0 {
				someRunning = true
			}
		} else {
			slog.Info("found running download", "name", torrent.Name, "state", torrent.State, "progress", torrent.Progress)
			someRunning = true
		}
	}
	if !someRunning {
		panic("no running downloads")
	}
}

type Torrent struct {
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Progress percent `json:"progress"`
	Hash     string  `json:"hash"`
	
	LastActivity int `json:"last_activity"`
}

func getDownloadLists() ([]*Torrent, error) {
	resp, err := http.Get("http://localhost:8080/api/v2/torrents/info?filter=downloading")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	var torrents []*Torrent
	err = json.NewDecoder(resp.Body).Decode(&torrents)
	if err != nil {
		return nil, err
	}
	return torrents, nil
}

func getTorrentPeers(hash string) (int, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/api/v2/sync/torrentPeers?hash=%s", hash))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("status code %d", resp.StatusCode)
	}

	var peerResp struct {
		Peers map[string]struct{} `json:"peers"`
	}
	err = json.NewDecoder(resp.Body).Decode(&peerResp)
	if err != nil {
		return 0, err
	}
	return len(peerResp.Peers), nil
}

type percent float64

var _ slog.LogValuer = percent(0)

// LogValue implements slog.LogValuer.
func (p percent) LogValue() slog.Value {
	return slog.StringValue(fmt.Sprintf("%.2f%%", p*100))
}
