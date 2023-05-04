package core

import (
	"encoding/hex"
	"testing"
)

func TestParseTorrentFile(t *testing.T) {

	torrentInfo, err := ParseTorrentFile("../res/sintel_trailer-480p.torrent")
	if err != nil {
		t.Errorf("parse error : %s", err)
	}

	infoHash := "27994de22087154c8245c68a12297a3079e6b67d"
	numPieces := 9
	pieceLength := 524288
	lastPieceLength := 178069
	numFiles := 1

	h := hex.EncodeToString(torrentInfo.InfoHash)
	if h != infoHash {
		t.Errorf("infoHash :  got %s, wanted %s", h, infoHash)
	}
	if torrentInfo.NumPieces != int64(numPieces) {
		t.Errorf("numPieces : got %d, wanted %d", torrentInfo.NumPieces, numPieces)
	}
	if torrentInfo.PieceLength != int64(pieceLength) {
		t.Errorf("pieceLength : got %d, wanted %d", torrentInfo.PieceLength, pieceLength)
	}
	if torrentInfo.LastPieceLength != int64(lastPieceLength) {
		t.Errorf("lastPieceLength : got %d, wanted %d", torrentInfo.LastPieceLength, lastPieceLength)
	}
	if len(torrentInfo.Files) != numFiles {
		t.Errorf("numFiles : got %d, wanted %d", len(torrentInfo.Files), numFiles)
	}
}
