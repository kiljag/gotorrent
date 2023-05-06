package main

import (
	gtc "core"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
)

func TestTracker() {

	announce := "http://localhost:9000/announce"
	// infoHash, _ := hex.DecodeString("27994de22087154c8245c68a12297a3079e6b67d")
	infoHash, _ := hex.DecodeString("3cdeeb871dc9a2c0aaf646b93afe1bd7e16b46c5")
	filelen := 524288

	clientId := gtc.GeneratePeerId()
	port := 6581

	req := &gtc.AnnounceReq{
		InfoHash:   infoHash,
		PeerId:     clientId,
		Port:       port,
		Uploaded:   0,
		Downloaded: 0,
		Left:       filelen,
		Compact:    1,
		Event:      gtc.EVENT_STARTED,
	}

	tracker := gtc.NewTracker(announce)
	res, err := tracker.GetAnnounce(req)
	if err != nil {
		panic(err)
	}

	fmt.Println("interval : ", res.Interval)
	fmt.Println("min interval : ", res.MinInterval)
	fmt.Println("complete : ", res.Complete)
	fmt.Println("incomplete : ", res.Incomplete)

	numPeers := len(res.Peers) / 6
	fmt.Println("numPeers : ", numPeers)

	for i := 0; i < numPeers; i++ {
		b := res.Peers[i*6 : i*6+6]
		ip := net.IP(b[:4])
		port := binary.BigEndian.Uint16(b[4:])
		fmt.Printf("peer : %s:%d\n", ip, port)
	}
}

func TestTorrent() {

	// torrentInfo, err := gtc.ParseTorrentFile("../res/sintel_trailer-480p.torrent")
	torrentInfo, err := gtc.ParseTorrentFile("../res/sample-trailers.torrent")

	// fmt.Printf("%+v\n", torrentInfo)
	// fmt.Printf("%+v\n", torrentInfo.Files)
	if err != nil {
		panic(err)
	}

	torrent := gtc.NewTorrent(torrentInfo)
	tChannel, err := torrent.Start()
	if err != nil {
		panic(err)
	}

	<-tChannel
}

func TestPeer() {

	// tinfo, err := gtc.ParseTorrentFile("../res/sintel_trailer-480p.torrent")
	// tinfo, err := gtc.ParseTorrentFile("../res/trailer-blender.torrent")
	tinfo, err := gtc.ParseTorrentFile("../res/sample-trailers.torrent")
	if err != nil {
		panic(err)
	}

	ip := net.IP([]byte{127, 0, 0, 1})
	port := uint16(63454)
	clientId := gtc.GeneratePeerId()
	reserved := make([]byte, 8)
	reserved[5] = reserved[5] & 0x10

	peerInfo, err := gtc.PeerHandshake(ip, port, tinfo.InfoHash, reserved, clientId)
	if err != nil {
		panic(err)
	}

	fmt.Printf("reserved : %x\n", peerInfo.Reserved)
	fmt.Printf("peer key : %d\n", peerInfo.Key)

	// send messages

}

func main() {
	// TestTracker()
	// TestPeer()
	TestTorrent()
}
