package test

import (
	"fmt"

	gtc "github.com/kiljag/gotorrent/core"
)

func TestTracker() {

	clientId := gtc.GeneratePeerId()
	clientPort := uint16(6541)

	torrentInfo, err := gtc.ParseTorrentFile("./res/sintel-trailer.torrent")
	if err != nil {
		panic(err)
	}

	tinfo := &gtc.TorrentInfo{
		InfoHash: torrentInfo.InfoHash,
		Length:   torrentInfo.Length,
	}

	fmt.Println("announce : ", torrentInfo.Announce)
	fmt.Printf("infoHash : %x\n", torrentInfo.InfoHash)

	tracker := gtc.NewTracker(torrentInfo.Announce, clientId, clientPort, tinfo)
	trackerChannel := make(chan *gtc.TrackerMessage)
	tracker.Start(trackerChannel)

	tm := <-trackerChannel
	switch tm.Type {
	case gtc.TMSG_PEER_LIST:
		v := tm.Data.([]*gtc.PeerInfo)
		for _, pv := range v {
			fmt.Printf("peer : %s:%d\n", pv.Ip, pv.Port)
		}
	default:
		fmt.Println("unrecognized tracker message type")
	}
}
