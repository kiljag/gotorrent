package test

import (
	"fmt"
	"net"

	gtc "github.com/kiljag/gotorrent/core"
)

func TestPeer() {

	tinfo, err := gtc.ParseTorrentFile("./res/sintel-trailer.torrent")
	if err != nil {
		panic(err)
	}
	fmt.Printf("fileName : %s\n", tinfo.Name)
	fmt.Printf("infoHash : %x\n", tinfo.InfoHash)

	ip, port := net.IP([]byte{127, 0, 0, 1}), uint16(63454)
	// ip, port := net.IP([]byte{192, 168, 0, 102}), uint16(63454)
	clientId := gtc.GeneratePeerId()
	reserved := make([]byte, 8)
	// reserved[5] |= 0x10
	fmt.Printf("clientReserved : %x\n", reserved)

	peerInfo := gtc.NewPeerInfo(ip, port)
	err = peerInfo.SendVerifyHandshake(tinfo.InfoHash, reserved, clientId)
	if err != nil {
		panic(err)
	}

	fmt.Printf("reserved : %x\n", peerInfo.Reserved)
	fmt.Printf("peer key : %s\n", peerInfo.Key)
	fmt.Println("peer handshake completed")
}
