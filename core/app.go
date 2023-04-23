package core

type TorrentManager struct {
}

func NewTorrentManager() *TorrentManager {
	return &TorrentManager{}
}

// will handle both magnet link and .torrent path
func (tm *TorrentManager) AddTorrent(tpath string) (chan interface{}, error) {
	torrentInfo, err := ParseTorrentFile(tpath)
	if err != nil {
		return nil, err
	}

	torrent := NewTorrent(torrentInfo)
	tChannel := torrent.start()

	return tChannel, nil
}
