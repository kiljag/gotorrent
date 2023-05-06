package core

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const (
	EVENT_STARTED   = "started"   // sent when the download is starting
	EVENT_STOPPED   = "stopped"   // sent when the download is stopped in between gracefully
	EVENT_COMPLETED = "completed" // sent when the download is completed
	EVENT_IGNORED   = ""          // dummy flag
)

type Tracker struct {
	Announce  string
	HostName  string
	Port      string
	TrackerID string
}

func NewTracker(Announce string) *Tracker {
	t := &Tracker{
		Announce: Announce,
	}
	u, _ := url.Parse(Announce)
	t.HostName = u.Hostname()
	t.Port = u.Port()

	return t
}

// wrapper methor to call announce on tracker, supports http, udp, tcp
func (tr *Tracker) GetAnnounce(params *AnnounceReq) (*AnnounceRes, error) {
	u, err := url.Parse(tr.Announce)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if u.Scheme == "http" || u.Scheme == "https" {
		res, err := tr.httpGetAnnounce(params)
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	return nil, nil
}

func (tr *Tracker) httpGetAnnounce(params *AnnounceReq) (*AnnounceRes, error) {
	rparams := url.Values{
		"info_hash":  []string{string(params.InfoHash)},
		"peer_id":    []string{string(params.PeerId)},
		"port":       []string{strconv.Itoa(params.Port)},
		"uploaded":   []string{strconv.Itoa(params.Uploaded)},
		"downloaded": []string{strconv.Itoa(params.Downloaded)},
		"compact":    []string{strconv.Itoa(params.Compact)},
		"left":       []string{strconv.Itoa(params.Left)},
	}
	if params.Event != "" {
		rparams.Add("event", params.Event)
	}

	req, err := url.Parse(tr.Announce)
	if err != nil {
		return nil, err
	}
	req.RawQuery = rparams.Encode()
	fmt.Println("GET : ", req)

	// make http request
	res, err := http.Get(req.String())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// bdecode response
	dec, err := BDecode(body)
	if err != nil {
		return nil, err
	}

	// populate tracker response object
	dmap := dec.(map[string]interface{})
	if v, ok := dmap["failure reason"]; ok {
		return nil, fmt.Errorf("%s", v.(string))
	}

	result := &AnnounceRes{}
	for k, v := range dmap {
		switch k {
		case "complete":
			result.Complete = v.(int64)
		case "incomplete":
			result.Incomplete = v.(int64)
		case "interval":
			result.Interval = v.(int64)
		case "min interval":
			result.MinInterval = v.(int64)
		case "peers":
			vstr := v.(string)
			result.Peers = []byte(vstr)
		case "peers_v6":
			vstr := v.(string)
			result.PeersV6 = []byte(vstr)

		default:
			fmt.Println("ignored : ", k)
		}
	}

	return result, nil
}
