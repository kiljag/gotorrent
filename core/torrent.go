package core

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	PROTOCOL = "BitTorrent protocol"
)

type Torrent struct {
	// info
	torrentInfo    *TorrentInfo
	torrentChannel chan interface{} // to send out messages, alerts etc.
	clientId       []byte           // self generated client id (20 bytes)
	clientPort     uint16
	clientReserved []byte // client reserved bits, 8 bytes

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
	fmt.Println("file length : ", torrentInfo.Length)
	fmt.Println("piece length : ", torrentInfo.PieceLength)
	fmt.Println("num pieces :", torrentInfo.NumPieces)
	fmt.Println()

	t := &Torrent{
		torrentInfo:    torrentInfo,
		torrentChannel: make(chan interface{}),
		clientId:       GeneratePeerId(),
		clientPort:     0,
		clientReserved: make([]byte, 8),

		trackerMap:     make(map[string]*Tracker, 0),
		trackerChannel: make(chan interface{}),

		peerInfoMap:   make(map[uint64]*PeerInfo),
		activePeerMap: make(map[uint64]*Peer),
		peerChannel:   make(chan *MessageParams),

		pieceManager:     NewPieceManager(torrentInfo),
		pieceMap:         make(map[int]bool),
		pieceReplication: make(map[int]int),
	}

	// extension protocol
	t.clientReserved[5] |= 0x10
	// t.clientReserved[7] |= 0x05

	return t
}

func (t *Torrent) Start() (chan interface{}, error) {
	// start a tcp server to handle peer requests

	go t.start()
	return t.torrentChannel, nil
}

func (t *Torrent) Stop() {

}

func (t *Torrent) Pause() {

}

func (t *Torrent) Resume() {

}

func (t *Torrent) start() {

	go t.pieceManager.start()
	go t.handlePeerChannel()
	go t.handleTrackerChannel()

	pbytes := []byte{127, 0, 0, 1, 247, 222}
	ip := net.IP(pbytes[:4])
	port := binary.BigEndian.Uint16(pbytes[4:6])
	peerInfo, err := PeerHandshake(ip, port,
		t.torrentInfo.InfoHash, t.clientReserved, t.clientId)
	if err != nil {
		panic(err)
	}
	// fmt.Println("client reserved ", hex.EncodeToString(t.clientReserved))
	fmt.Printf("creating new peer : %s -> %s\n",
		peerInfo.Conn.LocalAddr(), peerInfo.Conn.RemoteAddr())
	peer := NewPeer(peerInfo, t.torrentInfo, t.pieceManager)

	peer.Start(t.peerChannel)

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

	// handle magnet link
	if t.torrentInfo.IsFromMagnetLink {
		fmt.Println("is from magnet link")
	}

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
			chash := sha1.Sum(piece.data[0:piece.length])

			if bytes.Equal(phash, chash[:]) {
				fmt.Printf("got piece %d, ..verified\n", pi)
				t.pieceManager.savePiece(int(pi))
				downloaded++
			} else {
				fmt.Printf("got piece %d, Hash Mismatch : got(%x), wanted(%x)\n",
					pi, chash, phash)
			}

		case MESSAGE_PIECE_CANCELLED:

		case MESSAGE_BITFIELD:

		case MESSAGE_HAVE:
		}
	}
}

// // recv and send handshake, returns (peerId, error)
// func (t *Torrent) VerifyHandshake(conn net.Conn) ([]byte, error) {

// 	rh, err := t.recvHandShake(conn)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if rh.Pstr != PROTOCOL {
// 		return nil, fmt.Errorf("invalid protocol string %s", rh.Pstr)
// 	}

// 	// TODO : see if we have the file with infoHash

// 	peerId := make([]byte, 20)
// 	copy(peerId, rh.PeerId)
// 	sh := &HandShakeParams{
// 		PStrLen:  uint8(len(PROTOCOL)),
// 		Pstr:     PROTOCOL,
// 		Reserved: make([]byte, 8),
// 		InfoHash: rh.InfoHash,
// 		PeerId:   t.clientId,
// 	}
// 	peerInfo := &PeerInfo{}
// 	err = peerInfo.SendHandshake(sh)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return peerId, nil
// }
