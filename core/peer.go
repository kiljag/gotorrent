package core

import (
	"crypto/sha1"
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
	MESSAGE_PIECE          = 0x07
	MESSAGE_CANCEL         = 0x08
	MESSAGE_PORT           = 0x09

	// custom types
	MESSAGE_KEEPALIVE = 0x10
	MESSAGE_CONNECTED = 0x20

	// piece queue consts
	BLOCK_LENGTH       = 16 * 1024 // 16KB
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
	msgIdMap[MESSAGE_PIECE] = "piece"
	msgIdMap[MESSAGE_CANCEL] = "cancel"
	msgIdMap[MESSAGE_PORT] = "port"
	msgIdMap[MESSAGE_KEEPALIVE] = "keepalive"

	return msgIdMap
}

type Peer struct {
	conn   net.Conn
	tinfo  *TorrentInfo // torrent info
	key    uint64       // key to uniquely identify peer
	peerId []byte       // 20 bytes

	// peer state
	am_choking      bool // this client is choking the peer
	am_interested   bool // this client is intereted in the peer
	peer_choking    bool // peer is choking this client
	peer_interested bool // peer is interested in this client
	bitField        []byte

	// peer stats
	downloaded uint64 // bytes downloaded from this peer
	uploaded   uint64 // bytes uploaded to this peer

	// piece queue
	pieceChannel chan int       // channel to send downloaded piece indices
	pieceQueue   []int          // list of piece indices
	pieceMap     map[int]*Piece // piece index -> piece data

	// internal channels
	blocksFromPeer   chan *MessagePiece   // channel to receive blocks from peer
	requestsFromPeer chan *MessageRequest // channel to receive block requests from peer
	toPeer           chan *MessageParams  // messages to peer

	// utlity maps
	msgIdMap map[uint8]string
}

func NewPeer(conn net.Conn, peerId []byte, tinfo *TorrentInfo, pieceChannel chan int) *Peer {

	// generate key
	kbytes := sha1.Sum([]byte(conn.RemoteAddr().String()))
	var key uint64
	binary.BigEndian.PutUint64(kbytes[:8], key)

	// initialize bitfield
	bl := tinfo.NumPieces / 8
	if tinfo.NumPieces%8 != 0 {
		bl++
	}
	bitField := make([]byte, bl)

	p := &Peer{
		conn:   conn,
		tinfo:  tinfo,
		key:    key,
		peerId: peerId,

		am_choking:      true,
		am_interested:   false,
		peer_choking:    true,
		peer_interested: false,
		bitField:        bitField,

		downloaded: 0,
		uploaded:   0,

		pieceChannel: pieceChannel,
		pieceQueue:   make([]int, 0),
		pieceMap:     make(map[int]*Piece),

		blocksFromPeer:   make(chan *MessagePiece),
		requestsFromPeer: make(chan *MessageRequest),
		toPeer:           make(chan *MessageParams),

		msgIdMap: createMsgMap(),
	}
	// copy(p.clientId, clientId)
	return p
}

// send client's bitfile to peer, right after init
func (p *Peer) Start(bitField []byte) chan *MessageParams {
	go p.start()
	p.toPeer <- &MessageParams{
		Type: MESSAGE_BITFIELD,
		Data: &MessageBitField{
			BitField: bitField,
		},
	}

	return p.toPeer
}

// add new piece to the Queue
func (p *Peer) AddPieceToQ(piece *Piece) {
	pi := piece.Index
	p.pieceQueue = append(p.pieceQueue, pi)
	p.pieceMap[pi] = piece
}

// remove scheduled piece from Queue
func (p *Peer) RemovePieceFromQ(piece *Piece) {
	pi := piece.Index
	delete(p.pieceMap, pi)
}

// if peer contains a particular piece
func (p *Peer) ContainsPiece(pi int) bool {
	return IsSet(p.bitField, pi)
}

// Goroutine : handle piece requests from peer
func (p *Peer) handleRequestsFromPeer() {

	for {
		_, ok := <-p.requestsFromPeer
		if !ok {
			break
		}

		fmt.Println("received request from peer")
	}
}

// Goroutine :  handle piece requests to peer, job is to download the entire piece
func (p *Peer) handleRequestsToPeer() {

	for {
		// if there are no pieces in queue, wait.
		if len(p.pieceQueue) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		// ignore if piece is removed from queue, otherwise download
		pi := p.pieceQueue[0]
		err := p.getPiece(pi)
		if err != nil {
			fmt.Printf("error downloading piece %d : %s\n", pi, err)
		} else {
			p.pieceChannel <- pi
		}

		// increment offset
		p.pieceQueue = p.pieceQueue[1:]
	}
}

// get a piece block wise
func (p *Peer) getPiece(pi int) error {

	piece := p.pieceMap[pi]
	piece.Status = PIECE_IN_PROGRESS
	piece.ScheduledTime = time.Now().Unix()

	// prepare block requests
	reqs := make([]*MessageRequest, 0)
	for off := 0; off < piece.Length; off += BLOCK_LENGTH {
		blen := BLOCK_LENGTH
		if piece.Length-off < blen {
			blen = piece.Length - off
		}
		m := &MessageRequest{
			PieceIndex: uint32(piece.Index),
			Begin:      uint32(off),
			Length:     uint32(blen),
		}
		reqs = append(reqs, m)
	}

	// contains unfullfilled block requests
	requestQ := make(map[uint32]*MessageRequest, 0) // block offset -> request

	for len(reqs) > 0 || len(requestQ) > 0 {

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

		// check if peer sent blocks
		select {
		case msg := <-p.blocksFromPeer:
			delete(requestQ, msg.Begin)

		case <-time.After(500 * time.Millisecond):
		}
	}

	piece.Status = PIECE_COMPLETED
	piece.CompletionTime = time.Now().Unix()
	return nil
}

// peer state machine
func (p *Peer) start() {

	go p.handleRequestsToPeer()
	go p.handleRequestsFromPeer()

	for {
		// if the tcp socket is readable, read 4 bytes (msg len) with deadline
		buf := make([]byte, 4)
		p.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, _ := p.conn.Read(buf[:4])
		if n != 0 {
			if n > 0 && n < 4 {
				RecvNBytes(p.conn, buf[n:4])
			}

			mlen := binary.BigEndian.Uint32(buf[:4])
			msg, err := p.recvMessage(mlen)
			if err != nil {
				fmt.Println(err)
			}
			if msg != nil {
				switch msg.Type {
				case MESSAGE_PIECE:
					v := msg.Data.(*MessagePiece)
					p.blocksFromPeer <- v

				case MESSAGE_REQUEST:
					v := msg.Data.(*MessageRequest)
					p.requestsFromPeer <- v
				}
			}
		}

		// if there are messages to be sent to peer
		toPeerClosed := false
		select {
		case v, ok := <-p.toPeer:
			if !ok {
				fmt.Println("toPeer is closed")
				toPeerClosed = true
			} else {
				p.sendMessage(v)
			}
		default:
		}

		if toPeerClosed {
			break
		}
	}
}

// send message to peer, update relevant fields accordingly
func (p *Peer) sendMessage(message *MessageParams) error {

	if message.Type == MESSAGE_KEEPALIVE {
		k := make([]byte, 4)
		binary.BigEndian.PutUint32(k, uint32(0))
		return SendNBytes(p.conn, k)
	}

	// fmt.Printf("msg type : %d\n", message.Type)

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

	case MESSAGE_PIECE:
		v := message.Data.(*MessagePiece)
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
	// fmt.Println("recvMessage ::")
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
	RecvNBytes(p.conn, buf[0:1])
	mtype := buf[0]
	message := &MessageParams{
		Length: mlen,
		Type:   mtype,
		Data:   nil,
	}

	// handle bitfield message
	if mtype == MESSAGE_BITFIELD {
		RecvNBytes(p.conn, p.bitField)
		message.Data = &MessageBitField{
			BitField: p.bitField,
		}
		return message, nil
	}

	// handle piece message
	if mtype == MESSAGE_PIECE {
		headers := make([]byte, 8)
		RecvNBytes(p.conn, headers)
		pi := binary.BigEndian.Uint32(headers[0:4])
		begin := binary.BigEndian.Uint32(headers[4:8])
		len := mlen - 9
		var data []byte
		if piece, ok := p.pieceMap[int(pi)]; ok {
			data = piece.Data[begin : begin+len]
		} else {
			data = make([]byte, 16) // to be discarded
		}
		RecvNBytes(p.conn, data)
		message.Data = &MessagePiece{
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
		SetBit(p.bitField, int(pi))
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
