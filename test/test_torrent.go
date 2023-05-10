package test

import (
	"fmt"

	gtc "github.com/kiljag/gotorrent/core"
)

func TestTorrent() {

	torrentInfo, err := gtc.ParseTorrentFile("./res/sintel-trailer.torrent")

	if err != nil {
		panic(err)
	}

	fmt.Printf("info hash : %x\n", torrentInfo.InfoHash)
	fmt.Println("file length : ", torrentInfo.Length)
	fmt.Println("piece length : ", torrentInfo.PieceLength)
	fmt.Println("num pieces :", torrentInfo.NumPieces)
	fmt.Println("Announce List : ", torrentInfo.Announce)
	fmt.Println()

	torrent := gtc.NewTorrent(torrentInfo)
	tChannel := torrent.Start()
	<-tChannel
}
