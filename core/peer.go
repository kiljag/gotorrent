package core

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
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

	// custom types
	MESSAGE_KEEPALIVE         = 0x10
	MESSAGE_PEER_DISCONNECTED = 0x20
	MESSAGE_PIECE_COMPLETED   = 0x30
	MESSAGE_PIECE_CANCELLED   = 0x40

	// piece queue consts
	MAX_BLOCK_REQUESTS = 10
)

func createMsgMap() map[uint8]string {
	msgIdMap := make(map[uint8]string)
	msgIdMap[MESSAGE_CHOKE] = "choke"
	msgIdMap[MESSAGE_UNCHOKE] = "unchoke"
	msgIdMap[MESSAGE_INTERESTED] = "interested"
	msgIdMap[MESSAGE_NOT_INTERESTED] = "not+interested"
	msgIdMap[MESSAGE_HAVE] = "have"
	msgIdMap[MESSAGE_BITFIELD] = "bitfield"
	msgIdMap[MESSAGE_REQUEST] = "request"
	msgIdMap[MESSAGE_PIECE_BLOCK] = "piece+block"
	msgIdMap[MESSAGE_CANCEL] = "cancel"
	msgIdMap[MESSAGE_PORT] = "port"
	msgIdMap[MESSAGE_KEEPALIVE] = "keepalive"
	return msgIdMap
}

type Peer struct {
	conn         net.Conn
	key          uint64 // key to uniquely identify peer
	peerInfo     *PeerInfo
	torrentInfo  *TorrentInfo
	pieceManager *PieceManager

	// peer state
	am_choking      bool // this client is choking the peer
	am_interested   bool // this client is intereted in peer
	peer_choking    bool // peer is choking this client
	peer_interested bool // peer is interested in this client
	bitmap          *BitMap

	// peer stats
	keepalive  int64
	downloaded int64 // bytes downloaded from this peer
	uploaded   int64 // bytes uploaded to this peer

	// pieces to get from peer
	pieceQueue []int        // list of piece indices
	pieceMap   map[int]bool // piece index -> presence in Q

	// external channels
	peerChannel chan *MessageParams // channel to send downloaded messages outside

	// internal channels
	blocksFromPeer   chan *MessagePieceBlock // channel to receive blocks from peer
	requestsFromPeer chan *MessageRequest    // channel to receive block requests from peer
	toPeer           chan *MessageParams     // messages to peer

	// utlity maps
	msgIdMap map[uint8]string
}

func NewPeer(peerInfo *PeerInfo, torrentInfo *TorrentInfo, pieceManager *PieceManager) *Peer {

	p := &Peer{
		conn:         peerInfo.Conn,
		key:          peerInfo.Key,
		peerInfo:     peerInfo,
		torrentInfo:  torrentInfo,
		pieceManager: pieceManager,

		am_choking:      true,
		am_interested:   false,
		peer_choking:    true,
		peer_interested: false,
		bitmap:          NewBitMap(pieceManager.bitmap.length),

		keepalive:  0,
		downloaded: 0,
		uploaded:   0,

		pieceQueue: make([]int, 0),
		pieceMap:   make(map[int]bool),

		peerChannel: nil,

		blocksFromPeer:   make(chan *MessagePieceBlock),
		requestsFromPeer: make(chan *MessageRequest),
		toPeer:           make(chan *MessageParams),

		msgIdMap: createMsgMap(),
	}
	return p
}

func (p *Peer) start(peerChannel chan *MessageParams) {
	p.peerChannel = peerChannel
	go p.startPeer()
}

// wrapper method to read from peer
func (p *Peer) readFromPeer() (*MessageParams, error) {

	// if the tcp socket is readable, read first 4 bytes (msg len) with deadline
	buf := make([]byte, 4)
	p.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, _ := p.conn.Read(buf[:4])
	if n == 0 { // socket is not readable
		return nil, nil
	}

	// reset deadline for 1 minute, effectively blocking
	// p.conn.SetReadDeadline(time.Now().Add(1 * time.Minute))
	if n > 0 && n < 4 {
		fmt.Println("read mlen bytes")
		RecvNBytes(p.conn, buf[n:4])
	}
	mlen := binary.BigEndian.Uint32(buf[:4])
	if mlen > 256*1024 { // if mlen is too large, disconnect from peer
		return nil, fmt.Errorf("received mlen (%d) is too large", mlen)
	}

	msg, err := p.recvMessage(mlen)
	if err != nil {
		return nil, err
	}
	if msg.Type != MESSAGE_PIECE_BLOCK {
		fmt.Println("message from peer <= ", p.msgIdMap[msg.Type])
	}
	return msg, err
}

// wrapper method to write to peer
func (p *Peer) writeToPeer(msg *MessageParams) error {
	if msg.Type != MESSAGE_REQUEST {
		fmt.Println("message to peer =>  ", p.msgIdMap[msg.Type])
	}
	return p.sendMessage(msg)
}

// peer state machine
func (p *Peer) startPeer() {

	defer p.conn.Close()

	go p.handleRequestsToPeer()
	go p.handleRequestsFromPeer()

	// send client's bitfild to peer as first message
	p.writeToPeer(&MessageParams{
		Type: MESSAGE_BITFIELD,
		Data: &MessageBitField{
			BitField: p.pieceManager.bitmap.Bytes(),
		},
	})

	for {
		// send keepalive for every 2 minutes
		if time.Now().Unix()-int64(p.keepalive) > 120 {
			err := p.writeToPeer(&MessageParams{
				Type: MESSAGE_KEEPALIVE,
			})

			if err != nil {
				fmt.Println("peer error in sending keepalive")
			}
			p.keepalive = time.Now().Unix()
		}

		msg, err := p.readFromPeer()
		if err != nil {
			fmt.Println("peer error : ", err)
			break
		}

		if msg != nil {
			switch msg.Type {
			case MESSAGE_PIECE_BLOCK:
				v := msg.Data.(*MessagePieceBlock)
				p.blocksFromPeer <- v

			case MESSAGE_REQUEST:
				v := msg.Data.(*MessageRequest)
				p.requestsFromPeer <- v

			case MESSAGE_HAVE:
				p.peerChannel <- msg
			}
		}

		if p.peer_choking {
			fmt.Println("peer is choking")
			time.Sleep(5 * time.Second)
			continue
		}

		// if there are messages to be sent to peer
		select {
		case v := <-p.toPeer:
			p.writeToPeer(v)

		default:
		}
	}
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
			p.peerChannel <- &MessageParams{
				Type: MESSAGE_PIECE_CANCELLED,
				Data: &MessagePieceCancelled{
					PieceIndex: uint32(pi),
				},
			}
		} else {
			p.peerChannel <- &MessageParams{
				Type: MESSAGE_PIECE_COMPLETED,
				Data: &MessagePieceCompleted{
					PieceIndex: uint32(pi),
				},
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

// send message to peer, update relevant fields accordingly
func (p *Peer) sendMessage(message *MessageParams) error {

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

	case MESSAGE_CHOKE:
		p.am_choking = true

	case MESSAGE_UNCHOKE:
		p.am_choking = false

	case MESSAGE_INTERESTED:
		p.am_interested = true

	case MESSAGE_NOT_INTERESTED:
		p.am_interested = false

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

// read message from peer, update relelvant fields accordingly
func (p *Peer) recvMessage(mlen uint32) (*MessageParams, error) {

	buf := make([]byte, 32)
	if mlen == 0 {
		message := &MessageParams{
			Length: mlen,
			Type:   MESSAGE_KEEPALIVE,
			Data:   nil,
		}
		return message, nil
	}

	// recv 1 byte (message type param)
	err := RecvNBytes(p.conn, buf[0:1])
	if err != nil {
		return nil, err
	}
	mtype := buf[0]
	message := &MessageParams{
		Length: mlen,
		Type:   mtype,
		Data:   nil,
	}

	// handle bitfield message
	if mtype == MESSAGE_BITFIELD {
		err = RecvNBytes(p.conn, p.bitmap.Bytes())
		if err != nil {
			return nil, fmt.Errorf("bitfield error %s", err)
		}
		message.Data = &MessageBitField{
			BitField: p.bitmap.Bytes(),
		}
		return message, nil
	}

	// handle piece message
	if mtype == MESSAGE_PIECE_BLOCK {
		headers := make([]byte, 8)
		RecvNBytes(p.conn, headers)
		pi := binary.BigEndian.Uint32(headers[0:4])
		begin := binary.BigEndian.Uint32(headers[4:8])
		len := mlen - 9
		var data []byte
		if p.pieceMap[int(pi)] { // piece is in peer queue
			data = p.pieceManager.pieceMap[int(pi)].data[begin : begin+len]
		} else { // read the data to discard
			data = p.pieceManager.dummy.data[begin : begin+len]
		}
		RecvNBytes(p.conn, data)
		message.Data = &MessagePieceBlock{
			PieceIndex: pi,
			Begin:      begin,
			Block:      data,
		}
		return message, nil
	}

	var data []byte
	if mlen > 1 {
		data = make([]byte, mlen-1)
		RecvNBytes(p.conn, data)
	}

	// handle other messages
	switch message.Type {

	case MESSAGE_CHOKE:
		p.peer_choking = true

	case MESSAGE_UNCHOKE:
		p.peer_choking = false

	case MESSAGE_INTERESTED:
		p.peer_interested = true

	case MESSAGE_NOT_INTERESTED:
		p.peer_interested = false

	case MESSAGE_HAVE:
		pi := binary.BigEndian.Uint32(data[0:4])
		p.bitmap.SetBit(int(pi))
		message.Data = &MessageHave{
			PieceIndex: pi,
		}

	case MESSAGE_REQUEST:
		pi := binary.BigEndian.Uint32(data[0:4])
		begin := binary.BigEndian.Uint32(data[4:8])
		length := binary.BigEndian.Uint32(data[8:12])
		message.Data = &MessageRequest{
			PieceIndex: pi,
			Begin:      begin,
			Length:     length,
		}

	case MESSAGE_CANCEL:
		pi := binary.BigEndian.Uint32(data[0:4])
		begin := binary.BigEndian.Uint32(data[4:8])
		length := binary.BigEndian.Uint32(data[8:12])
		message.Data = &MessageCancel{
			PieceIndex: pi,
			Begin:      begin,
			Length:     length,
		}

	case MESSAGE_PORT:
		port := binary.BigEndian.Uint16(data[:2])
		message.Data = &MessagePort{
			Port: port,
		}
	}

	return message, nil
}
