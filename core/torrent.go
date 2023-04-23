package core

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type Torrent struct {
	// info
	torrentInfo    *TorrentInfo
	torrentChannel chan interface{} // to send out messages, alerts etc.
	clientId       []byte           // self generated client id (20 bytes)

	// trackers
	trackerMap     map[string]*Tracker // announce -> tracker
	trackerChannel chan interface{}    // to receive messages from trackers

	// peers
	peerInfoMap   map[uint64]*PeerInfo // peer -> peer
	activePeerMap map[uint64]*Peer     // peer.key -> peer
	peerChannel   chan *MessageParams  // to receive messages from peers

	// pieces
	pieceManager     *PieceManager
	pieceMap         map[int]bool // whether piece is scheduled or not
	pieceReplication map[int]int  // piece index -> replication count
}

func NewTorrent(torrentInfo *TorrentInfo) *Torrent {

	fmt.Printf("info hash : %x\n", torrentInfo.InfoHash)
	fmt.Println("file length : ", torrentInfo.FileSize)
	fmt.Println("piece length : ", torrentInfo.PieceLength)
	fmt.Println("num pieces :", torrentInfo.NumPieces)

	return &Torrent{
		torrentInfo:    torrentInfo,
		torrentChannel: make(chan interface{}),
		clientId:       GeneratePeerId(),

		trackerMap:     make(map[string]*Tracker, 0),
		trackerChannel: make(chan interface{}),

		peerInfoMap:   make(map[uint64]*PeerInfo),
		activePeerMap: make(map[uint64]*Peer),
		peerChannel:   make(chan *MessageParams),

		pieceManager:     NewPieceManager(torrentInfo),
		pieceMap:         make(map[int]bool),
		pieceReplication: make(map[int]int),
	}
}

func (t *Torrent) start() chan interface{} {
	go t.startTorrent()
	return t.torrentChannel
}

func (t *Torrent) startTorrent() {

	go t.pieceManager.start()
	go t.handlePeerChannel()
	go t.handleTrackerChannel()

	pbytes := []byte{127, 0, 0, 1, 247, 222}
	ip := net.IP(pbytes[:4])
	port := binary.BigEndian.Uint16(pbytes[4:6])
	peerInfo := &PeerInfo{
		Ip:   ip,
		Port: port,
		Key:  GeneratePeerKey(ip, port),
	}

	err := StartHandshake(peerInfo, t.torrentInfo.InfoHash, t.clientId)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("creating new peer : ", peerInfo.Conn.RemoteAddr().String())
	peer := NewPeer(peerInfo, t.torrentInfo, t.pieceManager)
	peer.start(t.peerChannel)

	for pi := 0; pi < int(t.torrentInfo.NumPieces); pi++ {
		peer.AddPieceToQ(pi)
	}

	<-time.After(5 * time.Minute)
	t.torrentChannel <- "done"
}

// process messages received from trackers
func (t *Torrent) handleTrackerChannel() {

}

// process messages received from peers
func (t *Torrent) handlePeerChannel() {

	downloaded := 0

	for {
		msg, ok := <-t.peerChannel
		if !ok {
			fmt.Println("piece channel is closed")
			break
		}

		switch msg.Type {
		case MESSAGE_PIECE_COMPLETED:
			v := msg.Data.(*MessagePieceCompleted)
			pi := v.PieceIndex
			// verify hash
			piece := t.pieceManager.pieceMap[int(pi)]
			phash := t.torrentInfo.PieceHashes[20*pi : 20*(pi+1)]
			chash := sha1.Sum(piece.data)

			if CompareBytes(phash, chash[:]) {
				fmt.Printf("got piece %d, .. verified\n", pi)
				t.pieceManager.completePiece(int(pi))
				downloaded++
			} else {
				fmt.Printf("Hash Mismatch : got(%x), wanted(%x)\n", chash, phash)
			}

		case MESSAGE_PIECE_CANCELLED:

		case MESSAGE_BITFIELD:

		case MESSAGE_HAVE:
		}
	}
}
