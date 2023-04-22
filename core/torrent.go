package core

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
)

type FileParams struct {
	InfoHash        []byte
	FileLength      int
	NumPieces       int
	PieceLength     int
	LastPieceLength int
	PieceHashes     []byte // len : NumPieces * 20
}

type Torrent struct {
	// info
	tinfo *TorrentInfo
	minfo *MagnetInfo

	// peers and pieces
	clientId     []byte // self generated client id (20 bytes)
	bitField     []byte
	alertChannel chan string
	trackers     map[string]*Tracker // announce -> tracker
	peers        map[string]*Peer    // ip -> peer
	pieceChannel chan int
	pieceMap     map[int]*Piece
}

func NewTorrent() *Torrent {
	return &Torrent{
		clientId:     GeneratePeerId(),
		bitField:     nil,
		alertChannel: make(chan string),

		trackers:     make(map[string]*Tracker, 0),
		peers:        make(map[string]*Peer, 0),
		pieceChannel: make(chan int),
		pieceMap:     make(map[int]*Piece),
	}
}

func (t *Torrent) AddTorrent(tinfo *TorrentInfo) {
	t.tinfo = tinfo
}

func (t *Torrent) AddMagnetLink(minfo *MagnetInfo) {
	t.minfo = minfo
}

func (t *Torrent) Start() chan string {
	go t.start()
	return t.alertChannel
}

// torrent state machine
func (t *Torrent) start() {

	// generate torrent params
	if t.tinfo == nil {
		panic("no torrent info given")
	}

	length := 0
	for _, f := range t.tinfo.Files {
		length += f.Length
	}

	fmt.Printf("info hash : %x\n", t.tinfo.InfoHash)
	fmt.Println("file length : ", t.tinfo.FileSize)
	fmt.Println("piece length : ", t.tinfo.PieceLength)
	fmt.Println("num pieces :", t.tinfo.NumPieces)

	// bitfield
	bl := t.tinfo.NumPieces / 8
	if t.tinfo.NumPieces%8 != 0 {
		bl++
	}
	t.bitField = make([]byte, bl)

	done := make(chan bool)
	go t.collectPieces(done)
	go t.schedulePieces(done)

	<-done
	<-done
	t.alertChannel <- "done"
}

func (t *Torrent) schedulePieces(done chan bool) {

	pbytes := []byte{127, 0, 0, 1, 247, 222}
	ip := net.IP(pbytes[:4])
	port := binary.BigEndian.Uint16(pbytes[4:6])
	conn, peerId, err := StartHandshake(ip, port, t.tinfo.InfoHash, t.clientId)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("creating new peer : ", conn.RemoteAddr().String())
	peer := NewPeer(conn, peerId, t.tinfo, t.pieceChannel)
	peer.Start(t.bitField)

	// pi := 0
	// piece := NewPiece(pi, t.tinfo.PieceLength)
	// t.pieceMap[pi] = piece
	// peer.AddPieceToQ(piece)

	for pi := 0; pi < int(t.tinfo.NumPieces); pi++ {
		pieceLength := t.tinfo.PieceLength
		offset := pi * t.tinfo.PieceLength
		if (t.tinfo.FileSize - uint64(offset)) < uint64(pieceLength) {
			pieceLength = int(t.tinfo.FileSize) - offset
		}

		piece := NewPiece(pi, pieceLength)
		t.pieceMap[pi] = piece
		peer.AddPieceToQ(piece)
	}

	done <- true
}

func (t *Torrent) collectPieces(done chan bool) {

	for {
		pi, ok := <-t.pieceChannel
		if !ok {
			fmt.Println("piece channel is closed")
			break
		}
		fmt.Println("got piece :", pi)

		// verify piece
		piece := t.pieceMap[pi]
		hoff := 20 * pi
		phash := t.tinfo.PieceHashes[hoff : hoff+20]
		chash := sha1.Sum(piece.Data)

		if CompareBytes(phash, chash[:]) {
			fmt.Printf("verified piece hash for : %d\n", pi)
		} else {
			fmt.Printf("Hash Mismatch : got(%x), wanted(%x)\n", chash, phash)
		}
	}
	done <- true
}
