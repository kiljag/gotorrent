package main

import (
	gtc "core"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func TestParseMagnetLink() {
	bytes, err := os.ReadFile("../mlink.txt")
	check(err)
	mlink := strings.TrimSpace(string(bytes))
	minfo, err := gtc.ParseMagnetLink(mlink)
	check(err)

	fmt.Println("dn : ", minfo.DisplayName)
	fmt.Println("xl : ", minfo.ExactLength)
	fmt.Printf("hash : %x\n", minfo.InfoHash)
	for _, tr := range minfo.AnnounceList {
		fmt.Println(tr)
	}
}

func TestParseTorrentFile() {
	tinfo, err := gtc.ParseTorrentFile("../ubuntu-22.10-desktop-amd64.iso.torrent")
	if err != nil {
		panic(err)
	}
	fmt.Println("announce : ", tinfo.Announce)
	fmt.Println("announce-list : ", tinfo.AnnounceList)
	fmt.Println("creation date : ", tinfo.CreationDate)
	fmt.Println("piece length : ", tinfo.PieceLength)
	fmt.Println("num pieces : ", len(tinfo.PieceHashes))
	fmt.Println("name : ", tinfo.Name)
	fmt.Println("#pieces : ", len(tinfo.PieceHashes)/20)
	fmt.Printf("hash %x\n", tinfo.InfoHash)
	for _, f := range tinfo.Files {
		fmt.Printf("fileInfo : path : (%s), len (%d)\n", f.Path, f.Length)
	}
}

func TestTrackerGet() {

	announce := "https://torrent.ubuntu.com/announce"
	infoHash, _ := hex.DecodeString("99c82bb73505a3c0b453f9fa0e881d6e5a32a0c1")
	filelen := 4071903232

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
	fmt.Println("#peers : ", len(res.Peers)/6)
	for i := 0; i < len(res.Peers)/6; i++ {
		fmt.Println("peer : ", res.Peers[i*6:i*6+6])
	}

	os.WriteFile("peers", res.Peers, 0644)
}

func PeerHandShake() {

	pbytes := []byte{127, 0, 0, 1, 247, 222}
	infoHash, _ := hex.DecodeString("27994de22087154c8245c68a12297a3079e6b67d")
	ip := net.IP(pbytes[:4])
	port := binary.BigEndian.Uint16(pbytes[4:])
	clientId := gtc.GeneratePeerId()

	peerInfo := &gtc.PeerInfo{
		Ip:   ip,
		Port: port,
	}

	err := gtc.StartHandshake(peerInfo, infoHash, clientId)
	if err != nil {
		panic(err)
	}
}

func TestTorrent() {

	tm := gtc.NewTorrentManager()
	// tChannel, err := tm.AddTorrent("../res/sintel_trailer-480p.mp4.torrent")
	tChannel, err := tm.AddTorrent("../res/sample-trailers.torrent")
	if err != nil {
		panic(err)
	}

	<-tChannel
}

func main() {
	TestTorrent()
}
