# GoTorrent : A BitTorrent client

## Usage

```golang

import (
	gtc "github.com/kiljag/gotorrent/core"
)

func main() {

	torrentInfo, err := gtc.ParseTorrentFile("./path/to/.torrent/file")
	if err != nil {
		panic(err)
	}

	torrent := gtc.NewTorrent(torrentInfo)
	tChannel := torrent.Start()
	<-tChannel
}

```