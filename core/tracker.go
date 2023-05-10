package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	EVENT_STARTED   = "started"   // sent when the download is starting
	EVENT_STOPPED   = "stopped"   // sent when the download is stopped in between gracefully
	EVENT_COMPLETED = "completed" // sent when the download is completed
	EVENT_IGNORED   = ""          // dummy flag

	UDP_PROTOCOL_ID = 0x41727101980 // magic constant

	ACTION_CONNECT  = 0
	ACTION_ANNOUNCE = 1
	ACTION_SCRAPE   = 2
	ACTION_ERROR    = 3

	TMSG_PEER_LIST = 0
)

// tracker struct
type Tracker struct {
	key      string
	announce string
	scheme   string
	hostname string
	ip       net.IP // tracker ip
	port     string
	event    string

	clientId       []byte
	clientPort     uint16
	tinfo          *TorrentInfo
	trackerChannel (chan *TrackerMessage)

	// tracker response
	seeders      int64
	leechers     int64
	interval     int64
	minInterval  int64
	trackerId    string
	externalIp   []byte
	connectionId uint64
	connectionAt int64
}

func NewTracker(announce string, clientId []byte, clientPort uint16, tinfo *TorrentInfo) *Tracker {

	u, err := url.Parse(announce)
	if err != nil {
		fmt.Println(announce, err)
	}

	key := fmt.Sprintf("(t)%s://%s", u.Scheme, u.Host)
	schemes := "udp::http::https"
	if !strings.Contains(schemes, u.Scheme) {
		fmt.Println(key, "unsupported tracker protocol")
	}

	t := &Tracker{
		key:        key,
		announce:   announce,
		scheme:     u.Scheme,
		hostname:   u.Hostname(),
		port:       u.Port(),
		tinfo:      tinfo,
		clientId:   clientId,
		clientPort: clientPort,
		event:      "started",

		connectionAt: 0,
	}

	if u.Scheme == "udp" {
		// get ip
		iplist, err := net.LookupIP(u.Hostname())
		if err != nil {
			fmt.Println(t.key, "lookupIP error")
		}
		for _, ip := range iplist {
			if len(ip) == 4 {
				t.ip = ip
				break
			}
		}
	}
	return t
}

// send
func (t *Tracker) Start(trackerChannel chan *TrackerMessage) {
	t.trackerChannel = trackerChannel

	if t.scheme == "udp" && t.ip == nil {
		return
	}
	go t.start()
}

func (t *Tracker) start() {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(t.key, "panic ", err)
		}
	}()

	for {
		event := 0
		if t.event == "started" {
			event = 1
			t.event = ""
		}

		switch t.scheme {
		case "http", "https":
			res, err := t.httpAnnounce()
			if err != nil {
				fmt.Println(err)
				break
			}

			// process response
			t.seeders = res.Complete
			t.leechers = res.Incomplete
			t.interval = res.Interval
			t.minInterval = res.MinInterval
			t.trackerId = res.TrackerId
			copy(t.externalIp[:], res.ExternalIP[:])

			t.trackerChannel <- &TrackerMessage{
				Key:  t.announce,
				Data: res.Peers,
			}

		case "udp":

			res, err := t.udpAnnounce(event)
			if err != nil {
				fmt.Println(err)
				break
			}

			t.seeders = int64(res.Seeders)
			t.leechers = int64(res.Leechers)
			t.interval = int64(res.Interval)

			t.trackerChannel <- &TrackerMessage{
				Key:  t.key,
				Type: TMSG_PEER_LIST,
				Data: res.Peers,
			}
		}

		// wait
		if t.interval <= 0 {
			t.interval = 300 // seconds
		}
		time.Sleep(time.Duration(t.interval) * time.Second)
	}
}

func (t *Tracker) httpAnnounce() (*AnnounceRes, error) {

	req := &AnnounceReq{
		Port:       t.clientPort,
		Uploaded:   0,
		Downloaded: 0,
		Left:       t.tinfo.Length,
		Compact:    1,
		Event:      t.event,
	}
	copy(req.InfoHash[:], t.tinfo.InfoHash)
	copy(req.PeerId[:], t.clientId)

	res, err := t.HTTPAnnounce(req)
	if err != nil {
		return nil, err
	}

	return res, err
}

func (t *Tracker) udpConnect() error {

	// get connectionId if expired
	if (time.Now().Unix() - t.connectionAt) > 110 {
		// fmt.Println("connecting to tracker...")

		// prepare request payload
		req := &ConnectReqUDP{
			ProtocolID:    UDP_PROTOCOL_ID,
			Action:        ACTION_CONNECT,
			TransactionId: GenerateTransactionId(),
		}

		res, err := t.UDPConnect(req)
		if err != nil {
			return err
		}

		// fmt.Println("got connectionId : ", res.ConnectionId)
		t.connectionId = res.ConnectionId
		t.connectionAt = time.Now().Unix()
	}

	return nil
}

func (t *Tracker) udpAnnounce(event int) (*AnnounceResUDP, error) {

	err := t.udpConnect()
	if err != nil {
		return nil, err
	}

	req := &AnnounceReqUDP{
		ConnectionId:  t.connectionId,
		Action:        ACTION_ANNOUNCE,
		TransactionId: GenerateTransactionId(),
		Downloaded:    uint64(0),
		Left:          uint64(t.tinfo.Length),
		Uploaded:      uint64(0),
		Event:         uint32(event),
		IpAddr:        0,
		Key:           0,
		NumWant:       -1,
		Port:          t.clientPort,
	}
	copy(req.InfoHash[:], t.tinfo.InfoHash)
	copy(req.PeerId[:], t.clientId)

	res, err := t.UDPAnnounce(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// connect to tracker, send payload and recv response
func (t *Tracker) UDP(req []byte) ([]byte, error) {
	addr := fmt.Sprintf("%s:%s", t.ip, t.port)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("tracker udp error : %s", err)
	}

	// send payload
	_, err = conn.Write(req)
	// fmt.Printf("tracker udp : written %d bytes\n", n)
	if err != nil {
		return nil, fmt.Errorf("error writing to udp socket : %s", err)
	}

	// read response
	res := make([]byte, 1024)
	n, err := conn.Read(res)
	// fmt.Printf("tracker udp : read %d bytes\n", n)
	if err != nil {
		return nil, fmt.Errorf("error reading from udp socket : %s", err)
	}

	return res[:n], nil
}

func (t *Tracker) UDPConnect(req *ConnectReqUDP) (*ConnectResUDP, error) {

	var payload [16]byte
	binary.BigEndian.PutUint64(payload[0:8], req.ProtocolID)
	binary.BigEndian.PutUint32(payload[8:12], req.Action)
	binary.BigEndian.PutUint32(payload[12:16], req.TransactionId)

	rbytes, err := t.UDP(payload[:])
	if err != nil {
		return nil, err
	}
	if len(rbytes) < 16 {
		return nil, fmt.Errorf("not enough bytes (%d) in connect res", len(rbytes))
	}

	action := binary.BigEndian.Uint32(rbytes[0:4])
	txnId := binary.BigEndian.Uint32(rbytes[4:8])

	if txnId != req.TransactionId {
		return nil, fmt.Errorf("mismatch in txn id, sent %d, recv %d",
			req.TransactionId, txnId)
	}

	if action != ACTION_CONNECT {
		return nil, fmt.Errorf("%s", string(rbytes[8:]))
	}

	res := &ConnectResUDP{
		Action:        action,
		TransactionId: txnId,
		ConnectionId:  binary.BigEndian.Uint64(rbytes[8:16]),
	}

	return res, nil
}

func (t *Tracker) UDPAnnounce(req *AnnounceReqUDP) (*AnnounceResUDP, error) {

	var payload [98]byte
	binary.BigEndian.PutUint64(payload[0:8], req.ConnectionId)
	binary.BigEndian.PutUint32(payload[8:12], req.Action)
	binary.BigEndian.PutUint32(payload[12:16], req.TransactionId)
	copy(payload[16:36], req.InfoHash[:])
	copy(payload[36:56], req.PeerId[:])
	binary.BigEndian.PutUint64(payload[56:64], req.Downloaded)
	binary.BigEndian.PutUint64(payload[64:72], req.Left)
	binary.BigEndian.PutUint64(payload[72:80], req.Uploaded)
	binary.BigEndian.PutUint32(payload[80:84], req.Event)
	binary.BigEndian.PutUint32(payload[84:88], req.IpAddr)
	binary.BigEndian.PutUint32(payload[88:92], req.Key)
	binary.BigEndian.PutUint32(payload[92:96], uint32(req.NumWant))
	binary.BigEndian.PutUint16(payload[96:98], req.Port)

	rbytes, err := t.UDP(payload[:])
	if err != nil {
		return nil, err
	}

	if len(rbytes) < 20 {
		return nil, fmt.Errorf("not enough bytes (%d) in announce res", len(rbytes))
	}

	action := binary.BigEndian.Uint32(rbytes[0:4])
	txnId := binary.BigEndian.Uint32(rbytes[4:8])

	if txnId != req.TransactionId {
		return nil, fmt.Errorf("mismatch in txn id, sent %d, recv %d",
			req.TransactionId, txnId)
	}

	if action != ACTION_ANNOUNCE {
		return nil, fmt.Errorf("%s", string(rbytes[8:]))
	}

	res := &AnnounceResUDP{
		Action:        action,
		TransactionId: txnId,
		Interval:      binary.BigEndian.Uint32(rbytes[8:12]),
		Leechers:      binary.BigEndian.Uint32(rbytes[12:16]),
		Seeders:       binary.BigEndian.Uint32(rbytes[16:20]),
	}

	pbytes := rbytes[20:]
	if len(pbytes)%6 != 0 {
		return nil, fmt.Errorf("invalid announce udp response")
	}
	res.Peers = make([]*PeerInfo, 0)
	for i := 0; i < len(pbytes); i += 6 {
		ip := net.IP(pbytes[i : i+4])
		port := binary.BigEndian.Uint16(pbytes[i+4 : i+6])
		res.Peers = append(res.Peers, NewPeerInfo(ip, port))
	}

	return res, nil
}

// send GET request to tracker and read response
func (t *Tracker) HTTP(params url.Values) ([]byte, error) {
	req, err := url.Parse(t.announce)
	if err != nil {
		return nil, fmt.Errorf("error parsing tracker announce url")
	}

	req.RawQuery = params.Encode()
	// fmt.Println("GET : ", req)
	res, err := http.Get(req.String())
	if err != nil {
		return nil, fmt.Errorf("error in http get to tracker")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error in reading tracker http response")
	}

	return body, nil
}

// wrapper methor to call announce on tracker, supports http, udp, tcp
func (t *Tracker) HTTPAnnounce(req *AnnounceReq) (*AnnounceRes, error) {

	params := url.Values{
		"info_hash":  []string{string(req.InfoHash[:])},
		"peer_id":    []string{string(req.PeerId[:])},
		"port":       []string{strconv.Itoa(int(req.Port))},
		"uploaded":   []string{strconv.Itoa(int(req.Uploaded))},
		"downloaded": []string{strconv.Itoa(int(req.Downloaded))},
		"left":       []string{strconv.Itoa(int(req.Left))},
		"compact":    []string{strconv.Itoa(req.Compact)},
	}
	if t.event == "" {
		params.Add("event", EVENT_STARTED)
	}

	// bdecode response
	body, err := t.HTTP(params)
	if err != nil {
		return nil, err
	}

	dec, err := BDecode(body)
	if err != nil {
		return nil, fmt.Errorf("bdecode error %s", err)
	}

	dmap := dec.(map[string]interface{})
	if v, ok := dmap["failure reason"]; ok {
		return nil, fmt.Errorf("failure in response : %s", v.(string))
	}

	res := &AnnounceRes{}

	if v, ok := dmap["complete"]; ok {
		res.Complete = v.(int64)
	}
	if v, ok := dmap["incomplete"]; ok {
		res.Incomplete = v.(int64)
	}
	if v, ok := dmap["interval"]; ok {
		res.Interval = v.(int64)
	}
	if v, ok := dmap["min interval"]; ok {
		res.MinInterval = v.(int64)
	}
	if v, ok := dmap["tracker id"]; ok {
		res.TrackerId = v.(string)
	}
	if v, ok := dmap["external ip"]; ok {
		copy(res.ExternalIP[:], []byte(v.(string)))
	}
	if v, ok := dmap["peers"]; ok {
		pbytes := []byte(v.(string))
		res.Peers = make([]*PeerInfo, 0)
		for i := 0; i < len(pbytes); i += 6 {
			ip := net.IP(pbytes[i : i+4])
			port := binary.BigEndian.Uint16(pbytes[i+4 : i+6])
			res.Peers = append(res.Peers, NewPeerInfo(ip, port))
		}
	}
	if _, ok := dmap["peers_v6"]; ok {
		fmt.Println("peers_v6 found")
	}

	return res, nil
}
