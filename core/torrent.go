package core

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"sync"
	"time"
)

const (
	PROTOCOL = "BitTorrent protocol"

	// expected peer messages
	PEER_PIECE_DONE   = 0
	PEER_PIECE_FAILED = 1
	PEER_META_INFO    = 2
	PEER_FAILED       = 3
)

type Torrent struct {
	// info
	tinfo          *TorrentInfo
	torrentChannel chan *TorrentMessage // to send out messages, alerts etc.
	clientId       []byte               // self generated client id (20 bytes)
	clientPort     uint16
	clientReserved []byte // client reserved bits, 8 bytes

	// trackers
	trackerMap     map[string]*Tracker  // announce -> tracker
	trackerChannel chan *TrackerMessage // to receive messages from trackers

	// peers
	peerLock      sync.Mutex
	peerInfoMap   map[string]*PeerInfo //
	inActivePeers map[string]*PeerInfo // peers which are disabled
	activePeerMap map[string]*Peer     // peer.key -> peer
	peerChannel   chan *PeerMessage    // to receive messages from peers

	// pieces
	pm        *PieceManager
	pieceLock sync.Mutex
	pieceMap  map[int]int // set status of piece, pending, scheduled, completed
}

func NewTorrent(tinfo *TorrentInfo) *Torrent {

	t := &Torrent{
		tinfo:          tinfo,
		torrentChannel: make(chan *TorrentMessage),
		clientId:       GeneratePeerId(),
		clientPort:     6541,
		clientReserved: make([]byte, 8),

		trackerMap:     make(map[string]*Tracker, 0),
		trackerChannel: make(chan *TrackerMessage),

		peerInfoMap:   make(map[string]*PeerInfo),
		inActivePeers: make(map[string]*PeerInfo),
		activePeerMap: make(map[string]*Peer),
		peerChannel:   make(chan *PeerMessage),

		pm:       NewPieceManager(tinfo),
		pieceMap: make(map[int]int),
	}

	// extension protocol
	// t.clientReserved[5] |= 0x10 // ltep extension

	return t
}

func (t *Torrent) Start() chan *TorrentMessage {
	// start a tcp server to handle peer requests

	go t.run()
	return t.torrentChannel
}

func (t *Torrent) Stop() {

}

func (t *Torrent) Pause() {

}

func (t *Torrent) Resume() {

}

// torrent state manager
func (t *Torrent) run() {

	// TODO: fetch metadata

	// bootstrap
	for i := 0; i < int(t.tinfo.NumPieces); i++ {
		t.pieceMap[i] = PIECE_PENDING
	}

	t.pm.Start()
	go t.handleTrackers()
	go t.handlePeers()

	for t.pm.downloadedPieces != int(t.tinfo.NumPieces) {

		fmt.Println("waiting for peer message..")
		msg, ok := <-t.peerChannel
		if !ok {
			fmt.Println("piece channel is closed")
			break
		}

		switch msg.Type {

		case PEER_PIECE_DONE:
			pi := msg.Data.(int)
			piece := t.pm.pieceMap[pi]
			phash := t.tinfo.PieceHashes[20*pi : 20*(pi+1)]
			chash := sha1.Sum(piece.data[0:piece.length])

			if bytes.Equal(phash, chash[:]) { // verify hash
				fmt.Printf("got piece %d/%d, ..verified\n", pi, t.tinfo.NumPieces)
				t.pm.savePiece(pi)

			} else {
				fmt.Printf("got piece %d, Hash Mismatch : got(%x), wanted(%x)\n",
					pi, chash, phash)
			}

		case PEER_PIECE_FAILED:
			pi := msg.Data.(int)
			t.pieceLock.Lock()
			t.pieceMap[pi] = PIECE_PENDING
			t.pieceLock.Unlock()
		}
	}

	fmt.Println("all pieces are downloaded, exiting..")
	t.torrentChannel <- &TorrentMessage{
		Type: 0,
		Data: "the end",
	}
}

// schedule pending pieces to peers
func (t *Torrent) handlePeers() {

	for t.pm.downloadedPieces != int(t.tinfo.NumPieces) {

		// check if there are any peers
		if len(t.peerInfoMap) == 0 {
			fmt.Println("no peers found, waiting..")
			time.Sleep(2 * time.Second)
			continue
		}

		// find pieces to schedule
		pendingPieces := make([]int, 0)
		t.pieceLock.Lock()
		for k, v := range t.pieceMap {
			if v == PIECE_PENDING {
				pendingPieces = append(pendingPieces, k)
				if len(pendingPieces) > 5 {
					break
				}
			}
		}
		t.pieceLock.Unlock()

		if (len(pendingPieces)) == 0 {
			fmt.Println("no pending pieces to schedule, waiting..")
			time.Sleep(2 * time.Second)
			continue
		}

		// initialize new peers
		initList := make([]*PeerInfo, 0)
		t.peerLock.Lock()
		for k, v := range t.peerInfoMap {
			if _, ok := t.activePeerMap[k]; ok {
				continue
			}
			if _, ok := t.inActivePeers[k]; ok {
				continue
			}
			initList = append(initList, v)
		}
		t.peerLock.Unlock()

		for _, pInfo := range initList {
			peerInfo := pInfo
			go func() {
				fmt.Println("initiating peer", peerInfo.Key)
				err := peerInfo.SendVerifyHandshake(t.tinfo.InfoHash, t.clientReserved, t.clientId)
				if err != nil {
					t.inActivePeers[peerInfo.Key] = peerInfo
					return
				}
				fmt.Println(peerInfo.Key, "connected")

				t.peerLock.Lock()
				peer := NewPeer(peerInfo, t.tinfo, t.pm)
				peer.Start(t.peerChannel)
				t.activePeerMap[peerInfo.Key] = peer
				t.peerLock.Unlock()
			}()
		}

		if len(initList) > 0 { // wait for new peers to connect
			fmt.Printf("initialized %d peers, waiting..\n", len(initList))
			time.Sleep(3 * time.Second)
		}

		if len(t.activePeerMap) == 0 {
			fmt.Printf("no active peers found, waiting..\n")
			time.Sleep(2 * time.Second)
			continue
		}

		// schedule pieces to peers (round robin)
		t.peerLock.Lock()
		t.pieceLock.Lock()

		fmt.Println("pending Pieces : ", pendingPieces)
		i := 0
		for i < len(pendingPieces) {
			for _, peer := range t.activePeerMap {
				if i < len(pendingPieces) {
					pi := pendingPieces[i]
					peer.AddPieceToQ(pi)
					t.pieceMap[pi] = PIECE_SCHEDULED
					fmt.Printf("scheduled %d to peer %s\n", pi, peer.key)
					i++
				}
			}
		}
		t.pieceLock.Unlock()
		t.peerLock.Unlock()

		time.Sleep(5 * time.Second)
	}

	t.torrentChannel <- &TorrentMessage{
		Type: 0,
		Data: "done",
	}
}

// process messages received from trackers
func (t *Torrent) handleTrackers() {

	// initialize trackers
	announceList := append([]string{t.tinfo.Announce}, t.tinfo.AnnounceList...)
	for _, announce := range announceList {
		// skip if the tracker exists
		if _, ok := t.trackerMap[announce]; ok {
			continue
		}
		tracker := NewTracker(announce, t.clientId, t.clientPort, t.tinfo)
		tracker.Start(t.trackerChannel)
		t.trackerMap[announce] = tracker
	}
	fmt.Printf("started %d trackers\n", len(announceList))

	for {
		tm, ok := <-t.trackerChannel
		if !ok {
			fmt.Println("trackerChannel is closed")
			break
		}

		switch v := tm.Data.(type) {
		case []*PeerInfo:
			fmt.Printf("recevied %d peers from %s\n", len(v), tm.Key)
			t.peerLock.Lock()
			for _, peerInfo := range v {
				// add to map if key is missing in peerInfo map
				if _, ok := t.peerInfoMap[peerInfo.Key]; !ok {
					t.peerInfoMap[peerInfo.Key] = peerInfo
				}
			}
			t.peerLock.Unlock()

		default:
			fmt.Printf("unknown tracker msg type : %T", v)
		}
	}
}

func (t *Torrent) DownloadedPieces() int {
	return int(t.pm.downloadedPieces)
}
