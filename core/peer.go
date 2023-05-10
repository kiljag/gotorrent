package core

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// core protocol
	MESSAGE_CHOKE          = 0x00
	MESSAGE_UNCHOKE        = 0x01
	MESSAGE_INTERESTED     = 0x02
	MESSAGE_NOT_INTERESTED = 0x03
	MESSAGE_HAVE           = 0x04
	MESSAGE_BITFIELD       = 0x05
	MESSAGE_REQUEST        = 0x06
	MESSAGE_PIECE_BLOCK    = 0x07
	MESSAGE_CANCEL         = 0x08
	MESSAGE_PORT           = 0x09

	// extension protocol
	MESSAGE_LTEP = 0x14

	// fast peers protocol
	MESSAGE_SUGGEST        = 0x0D
	MESSAGE_HAVE_ALL       = 0x0E
	MESSAGE_HAVE_NONE      = 0x0F
	MESSAGE_REJECT_REQUEST = 0x10
	MESSAGE_ALLOWED_FAST   = 0x11

	// hash transfer protocol
	MESSAGE_HASH_REQUEST = 0x15
	MESSAGE_HASHES       = 0x16
	MESSAGE_HASH_REJECT  = 0x17

	// custom types
	MESSAGE_KEEPALIVE         = 0xF0
	MESSAGE_PEER_DISCONNECTED = 0xF1
	MESSAGE_PIECE_COMPLETED   = 0xF2
	MESSAGE_PIECE_CANCELLED   = 0xF3

	// reserved
	RESERVED_EXTENSION = uint64(0x00100000)

	//
	MAX_BLOCK_REQUESTS = 10
)

type Peer struct {
	sync.Mutex
	conn         net.Conn
	key          string
	peerInfo     *PeerInfo
	torrentInfo  *TorrentInfo
	pieceManager *PieceManager
	bitmap       *BitMap

	// peer state
	am_choking      bool // this client is choking the peer
	am_interested   bool // this client is intereted in peer
	peer_choking    bool // peer is choking this client
	peer_interested bool // peer is interested in this client

	// peer stats
	keepalive  int64
	downloaded int64 // bytes downloaded from this peer
	uploaded   int64 // bytes uploaded to this peer

	// pieces to get from peer
	pieceQueue []int        // list of piece indices
	pieceMap   map[int]bool // piece index -> presence in Q

	peerChannel      chan *PeerMessage       // to send peer messages to torrent manager
	blocksFromPeer   chan *MessagePieceBlock // channel to receive blocks from peer
	requestsFromPeer chan *MessageRequest    // channel to receive block requests from peer
	toPeer           chan *MessageParams     // to send messages to peer

	// utlity maps
	msgIdMap map[uint8]string

	//extension
	extensionMap      map[string]uint8            // extension id's local to this peer
	extensionIdMap    map[uint8]string            // reverse mapping of extensions
	extensionProps    map[string]interface{}      // peer properties from handshake
	extensionChannels map[string]chan interface{} // message type to interface
}

func NewPeer(peerInfo *PeerInfo, torrentInfo *TorrentInfo, pieceManager *PieceManager) *Peer {

	p := &Peer{
		conn:         peerInfo.Conn,
		key:          peerInfo.Key,
		peerInfo:     peerInfo,
		torrentInfo:  torrentInfo,
		pieceManager: pieceManager,
		bitmap:       NewBitMap(pieceManager.bitmap.length),

		am_choking:      true,
		am_interested:   false,
		peer_choking:    true,
		peer_interested: false,

		keepalive:  0,
		downloaded: 0,
		uploaded:   0,

		pieceQueue: make([]int, 0),
		pieceMap:   make(map[int]bool),

		peerChannel: nil,

		blocksFromPeer:   make(chan *MessagePieceBlock),
		requestsFromPeer: make(chan *MessageRequest),
		toPeer:           make(chan *MessageParams),

		extensionMap:      make(map[string]uint8),
		extensionIdMap:    make(map[uint8]string),
		extensionProps:    make(map[string]interface{}),
		extensionChannels: make(map[string]chan interface{}),

		msgIdMap: createMsgMap(),
	}
	return p
}

func (p *Peer) Start(peerChannel chan *PeerMessage) {
	p.peerChannel = peerChannel
	go p.start()
}

// peer state machine
func (p *Peer) start() {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(p.key, "panic", err)
		}
	}()

	defer p.conn.Close()
	go p.handleRequestsToPeer()
	go p.handleRequestsFromPeer()

	go func() {
		// send extension handshake
		m := make(map[string]interface{})
		m[EXT_PEX] = 1
		m[EXT_UPLOAD_ONLY] = 2
		m[EXT_HOLEPUNCH] = 4

		payload := make(map[string]interface{})
		payload["m"] = m

		payload["reqq"] = 2000
		payload["upload_only"] = 1
		payload["v"] = "GoTorrent/1.0.0"
		payload["yourip"] = string([]byte{127, 0, 0, 1})

		_, err := BEncode(payload)
		if err != nil {
			panic(err)
		}

		// p.toPeer <- &MessageParams{
		// 	Type: MESSAGE_LTEP,
		// 	Data: &MessageExtension{
		// 		EType:   0x00,
		// 		Payload: enc,
		// 	},
		// }

		// send bitfield
		p.toPeer <- &MessageParams{
			Type: MESSAGE_BITFIELD,
			Data: &MessageBitField{
				BitField: p.pieceManager.bitmap.Bytes(),
			},
		}

		// send interested
		p.toPeer <- &MessageParams{
			Type: MESSAGE_INTERESTED,
		}
	}()

	go func() { // periodically send keepalive messages
		for {
			time.Sleep(2 * time.Minute)
			p.toPeer <- &MessageParams{
				Type: MESSAGE_KEEPALIVE,
			}
		}
	}()

	for {
		msg, err := p.recvMessage()
		if err != nil {
			fmt.Println(p.key, err)
			break
		}
		if msg != nil {
			fmt.Println(p.key, "from peer <= ", p.msgIdMap[msg.Type], msg.Type)
			switch msg.Type {

			case MESSAGE_CHOKE:
				p.peer_choking = true
			case MESSAGE_UNCHOKE:
				p.peer_choking = false
			case MESSAGE_INTERESTED:
				p.peer_interested = true
			case MESSAGE_NOT_INTERESTED:
				p.peer_interested = false

			case MESSAGE_HAVE:
				v := msg.Data.(*MessageHave)
				p.bitmap.SetBit(int(v.PieceIndex))

			case MESSAGE_BITFIELD:
				v := msg.Data.(*MessageBitField)
				p.bitmap.bitmap = v.BitField

			case MESSAGE_PIECE_BLOCK:
				v := msg.Data.(*MessagePieceBlock)
				p.blocksFromPeer <- v

			case MESSAGE_REQUEST:
				v := msg.Data.(*MessageRequest)
				p.requestsFromPeer <- v

			case MESSAGE_LTEP:
				ext := msg.Data.(*MessageExtension)
				p.handleExtensionMessage(ext)
			}
		}

		// if there are messages to be sent to peer
		select {
		case v := <-p.toPeer:
			fmt.Println(p.key, " toPeer => ", p.msgIdMap[v.Type], v.Type)
			p.sendMessage(v)

			switch v.Type {
			case MESSAGE_CHOKE:
				p.am_choking = true

			case MESSAGE_UNCHOKE:
				p.am_choking = false

			case MESSAGE_INTERESTED:
				p.am_interested = true

			case MESSAGE_NOT_INTERESTED:
				p.am_interested = false
			}

		default:
		}
	}
}

// non-blocking network io to read one message from peer
func (p *Peer) recvMessage() (*MessageParams, error) {

	// if the tcp socket is readable
	var buf [4]byte
	p.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, _ := p.conn.Read(buf[:])
	if n == 0 { // socket is not readable
		return nil, nil
	}

	if n < 4 {
		err := RecvNBytes(p.conn, buf[n:4])
		if err != nil {
			return nil, fmt.Errorf("error reading mlen")
		}
	}

	mlen := binary.BigEndian.Uint32(buf[:4])
	if mlen > 256*1024 {
		return nil, fmt.Errorf("received mlen (%d) is too large", mlen)
	}

	// keepalive message
	if mlen == 0 {
		message := &MessageParams{
			Length: mlen,
			Type:   MESSAGE_KEEPALIVE,
			Data:   nil,
		}
		return message, nil
	}

	// read message type
	err := RecvNBytes(p.conn, buf[:1])
	if err != nil {
		return nil, fmt.Errorf("error reading message type")
	}

	mtype := buf[0]
	message := &MessageParams{
		Length: mlen,
		Type:   mtype,
		Data:   nil,
	}

	if mlen == 1 {
		return message, nil
	}

	// handle piece block message
	if mtype == MESSAGE_PIECE_BLOCK {
		var headers [8]byte
		err := RecvNBytes(p.conn, headers[:])
		if err != nil {
			return nil, fmt.Errorf("error reading piece block headers")
		}

		pi := binary.BigEndian.Uint32(headers[:4])
		begin := binary.BigEndian.Uint32(headers[4:8])
		blen := mlen - 9 // block length

		var block []byte
		if p.pieceMap[int(pi)] { // piece is in peer queue
			block = p.pieceManager.pieceMap[int(pi)].data[begin : begin+blen]
		} else { // read the data to discard
			block = p.pieceManager.dummy.data[begin : begin+blen]
		}

		err = RecvNBytes(p.conn, block)
		if err != nil {
			return nil, fmt.Errorf("error reading block data")
		}

		message.Data = &MessagePieceBlock{
			PieceIndex: pi,
			Begin:      begin,
			Block:      block,
		}
		return message, nil
	}

	payload := make([]byte, mlen-1)
	err = RecvNBytes(p.conn, payload)
	if err != nil {
		return nil, fmt.Errorf("error reading message payload %s", err)
	}

	switch mtype {

	case MESSAGE_BITFIELD:
		message.Data = &MessageBitField{
			BitField: payload,
		}

	case MESSAGE_HAVE:
		message.Data = &MessageHave{
			PieceIndex: binary.BigEndian.Uint32(payload[0:4]),
		}

	case MESSAGE_REQUEST:
		message.Data = &MessageRequest{
			PieceIndex: binary.BigEndian.Uint32(payload[0:4]),
			Begin:      binary.BigEndian.Uint32(payload[4:8]),
			Length:     binary.BigEndian.Uint32(payload[8:12]),
		}

	case MESSAGE_CANCEL:
		message.Data = &MessageCancel{
			PieceIndex: binary.BigEndian.Uint32(payload[0:4]),
			Begin:      binary.BigEndian.Uint32(payload[4:8]),
			Length:     binary.BigEndian.Uint32(payload[8:12]),
		}

	case MESSAGE_PORT:
		message.Data = &MessagePort{
			Port: binary.BigEndian.Uint16(payload[:2]),
		}

	case MESSAGE_LTEP:
		message.Data = &MessageExtension{
			EType:   payload[0],
			Payload: payload[1:],
		}

	default:
		fmt.Println(p.key, "unrecognized mtype ", message.Type)
		message.Data = &MessageDefault{
			Payload: payload,
		}
	}

	return message, nil
}

// send message to peer, update relevant fields accordingly
func (p *Peer) sendMessage(message *MessageParams) error {

	// fmt.Println("  to peer => ", p.msgIdMap[message.Type])
	if message.Type == MESSAGE_KEEPALIVE {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(0))
		return SendNBytes(p.conn, k)
	}

	// prepare data payload
	payload := make([]byte, 5)
	payload[4] = message.Type // set message type
	data := make([]byte, 0)   // for bitfield and piece requests

	switch message.Type {

	case MESSAGE_HAVE:
		v := message.Data.(*MessageHave)
		h := make([]byte, 4)
		binary.BigEndian.PutUint32(h, v.PieceIndex)
		payload = append(payload, h...)

	case MESSAGE_BITFIELD:
		v := message.Data.(*MessageBitField)
		data = v.BitField

	case MESSAGE_REQUEST:
		v := message.Data.(*MessageRequest)
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h[0:4], v.PieceIndex)
		binary.BigEndian.PutUint32(h[4:8], v.Begin)
		binary.BigEndian.PutUint32(h[8:12], v.Length)
		payload = append(payload, h...)

	case MESSAGE_PIECE_BLOCK:
		v := message.Data.(*MessagePieceBlock)
		h := make([]byte, 8)
		binary.BigEndian.PutUint32(h[0:4], v.PieceIndex)
		binary.BigEndian.PutUint32(h[4:8], v.Begin)
		payload = append(payload, h...)
		data = v.Block

	case MESSAGE_CANCEL:
		v := message.Data.(*MessageCancel)
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h[0:4], v.PieceIndex)
		binary.BigEndian.PutUint32(h[4:8], v.Begin)
		binary.BigEndian.PutUint32(h[8:12], v.PieceIndex)
		payload = append(payload, h...)

	case MESSAGE_PORT:
		v := message.Data.(*MessagePort)
		h := make([]byte, 2)
		binary.BigEndian.PutUint16(h[0:4], v.Port)
		payload = append(payload, h...)

	case MESSAGE_LTEP:
		v := message.Data.(*MessageExtension)
		payload = append(payload, v.EType)
		payload = append(payload, v.Payload...)
		// fmt.Println(p.key, "payload : ", len(payload))

	}

	// update message length
	mlen := len(payload) + len(data) - 4
	binary.BigEndian.PutUint32(payload[0:4], uint32(mlen))
	err := SendNBytes(p.conn, payload)
	if err != nil {
		return err
	}

	// send piece data or bitmask
	if len(data) > 0 {
		err := SendNBytes(p.conn, data)
		if err != nil {
			return err
		}
	}

	return nil
}

// add new piece to the Queue
func (p *Peer) AddPieceToQ(pi int) {
	p.pieceQueue = append(p.pieceQueue, pi)
	p.pieceMap[pi] = true
}

// remove scheduled piece from Queue
func (p *Peer) RemovePieceFromQ(pi int) {
	delete(p.pieceMap, pi)
}

// if peer contains a particular piece
func (p *Peer) ContainsPiece(pi int) bool {
	return p.bitmap.IsSet(pi)
}

// Goroutine : handle piece block requests from peer
func (p *Peer) handleRequestsFromPeer() {
	for {
		_, ok := <-p.requestsFromPeer
		if !ok {
			break
		}
		fmt.Println("received request from peer")
	}
}

// Goroutine :  downloaded pieces from peer
func (p *Peer) handleRequestsToPeer() {

	for {
		// if there are no pieces in queue, wait.
		if len(p.pieceQueue) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		pi := p.pieceQueue[0]
		err := p.getPiece(pi)
		if err != nil {
			fmt.Printf("error downloading piece %d : %s\n", pi, err)
			p.peerChannel <- &PeerMessage{
				Type: PEER_PIECE_FAILED,
				Data: pi,
			}
		} else {
			p.peerChannel <- &PeerMessage{
				Type: PEER_PIECE_DONE,
				Data: pi,
			}
		}
		// increment offset
		p.pieceQueue = p.pieceQueue[1:]
	}
}

// get a piece block wise
func (p *Peer) getPiece(pi int) error {

	piece := p.pieceManager.getPiece(pi)

	// prepare block requests
	reqs := make([]*MessageRequest, 0)
	for off := 0; off < piece.length; off += PIECE_BLOCK_LENGTH {
		blen := PIECE_BLOCK_LENGTH
		if piece.length-off < blen {
			blen = piece.length - off
		}
		m := &MessageRequest{
			PieceIndex: uint32(pi),
			Begin:      uint32(off),
			Length:     uint32(blen),
		}
		reqs = append(reqs, m)
	}

	// contains unfullfilled block requests
	requestQ := make(map[uint32]*MessageRequest, 0) // block offset -> request

	for len(reqs) > 0 || len(requestQ) > 0 {
		// see if the piece is removed from queue
		if !p.pieceMap[pi] {
			return fmt.Errorf("piece %d is removed from queue", pi)
		}

		// send interested if not set
		if !p.am_interested {
			p.toPeer <- &MessageParams{
				Type: MESSAGE_INTERESTED,
			}
		}

		// wait for some time if peer is choking
		if p.peer_choking {
			time.Sleep(2 * time.Second)
			continue
		}

		// check if next block can be requested
		if len(reqs) > 0 && len(requestQ) < MAX_BLOCK_REQUESTS {
			req := reqs[0]
			requestQ[req.Begin] = req
			msg := &MessageParams{
				Type: MESSAGE_REQUEST,
				Data: req,
			}
			p.toPeer <- msg
			reqs = reqs[1:]
		}

		// check if blocks are sent by peer
		select {
		case msg := <-p.blocksFromPeer:
			if msg.PieceIndex == uint32(pi) {
				delete(requestQ, msg.Begin)
			}
		case <-time.After(500 * time.Millisecond):
		}
	}

	return nil
}
