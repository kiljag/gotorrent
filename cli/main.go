package main

import (
	gtc "core"
	"encoding/hex"
	"fmt"
	"net"
)

func TestTorrent() {

	torrentInfo, err := gtc.ParseTorrentFile("../res/sintel_trailer-480p.torrent")
	// torrentInfo, err := gtc.ParseTorrentFile("../res/sample-trailers.torrent")

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

	peerInfo, err := gtc.PeerHandshake(ip, port, "123", tinfo.InfoHash, reserved, clientId)
	if err != nil {
		panic(err)
	}

	fmt.Printf("reserved : %x\n", peerInfo.Reserved)
	fmt.Printf("peer key : %s\n", peerInfo.Key)

	// send messages
}

func TestTracker() {

	clientId := gtc.GeneratePeerId()
	clientPort := uint16(6541)

	// announce := "http://localhost:9000/announce"
	// tinfo, err := gtc.ParseTorrentFile("../res/sample-trailers.torrent")
	// if err != nil {
	// 	panic(err)
	// }

	announce := "http://open.acgnxtracker.com:80/announce"
	// announce := "udp://tracker.torrent.eu.org:451/announce"
	infoHash, _ := hex.DecodeString("c5ae0f24349e6006002bb46fd9c50a36d6a0fb3b")
	tinfo := &gtc.TorrentInfo{
		InfoHash: infoHash,
		Length:   383147546,
	}

	pm := gtc.NewPieceManager(tinfo)
	tracker, err := gtc.NewTracker(announce, clientId, clientPort, tinfo, pm)
	if err != nil {
		panic(err)
	}
	// fmt.Printf("tracker : %+v\n", tracker)

	// test connect
	trackerChannel := make(chan gtc.PeerV4List)
	tracker.Start(trackerChannel)

	for {
		v, ok := <-trackerChannel
		if !ok {
			break
		}
		for _, pv := range v {
			fmt.Printf("peer : %s:%d\n", pv.Ip, pv.Port)
		}
	}
}

func main() {

	// TestPeer()
	// TestTorrent()
	TestTracker()
}
