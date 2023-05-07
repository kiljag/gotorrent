package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
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

	// fast extensions
	MESSAGE_SUGGEST        = 0x0D
	MESSAGE_HAVE_ALL       = 0x0E
	MESSAGE_HAVE_NONE      = 0x0F
	MESSAGE_REJECT_REQUEST = 0x10
	MESSAGE_ALLOWED_FAST   = 0x11

	// ltep extension
	MESSAGE_EXTENSION_PROTOCOL = 0x14

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
	msgIdMap[MESSAGE_SUGGEST] = "suggest"
	msgIdMap[MESSAGE_HAVE_ALL] = "have+all"
	msgIdMap[MESSAGE_HAVE_NONE] = "have+none"
	msgIdMap[MESSAGE_REJECT_REQUEST] = "reject+request"
	msgIdMap[MESSAGE_ALLOWED_FAST] = "allowed+fast"
	msgIdMap[MESSAGE_EXTENSION_PROTOCOL] = "extension+protocol"
	msgIdMap[MESSAGE_HASH_REQUEST] = "hash+request"
	msgIdMap[MESSAGE_HASHES] = "hashes"
	msgIdMap[MESSAGE_HASH_REJECT] = "hash+reject"
	return msgIdMap
}

// peer info
type PeerInfo struct {
	Ip   net.IP
	Port uint16
	Key  string

	Conn     net.Conn
	PeerId   []byte
	Reserved []byte
}

func (p *PeerInfo) SendHandshake(h *HandShakeParams) error {
	payload := make([]byte, 0)
	payload = append(payload, h.PStrLen)
	payload = append(payload, []byte(h.Pstr)...)
	payload = append(payload, h.Reserved[:]...)
	payload = append(payload, h.InfoHash[:]...)
	payload = append(payload, h.PeerId[:]...)
	err := SendNBytes(p.Conn, payload)
	if err != nil {
		return err
	}
	return nil
}

// read handshake response
func (p *PeerInfo) RecvHandshake() (*HandShakeParams, error) {
	buf := make([]byte, 68)
	err := RecvNBytes(p.Conn, buf)
	if err != nil {
		fmt.Println()
		return nil, err
	}
	h := &HandShakeParams{
		Reserved: make([]byte, 8),
		InfoHash: make([]byte, 20),
		PeerId:   make([]byte, 20),
	}
	h.PStrLen = buf[0]
	h.Pstr = string(buf[1:20])
	copy(h.Reserved[:], buf[20:28])
	copy(h.InfoHash[:], buf[28:48])
	copy(h.PeerId, buf[48:68])
	return h, nil
}

// open a tcp connection with peer
// send and verify handshake
func PeerHandshake(ip net.IP, port uint16, key string,
	infoHash []byte, reserved []byte, clientId []byte) (*PeerInfo, error) {

	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("connected to peer %s -> %s\n", conn.LocalAddr(), conn.RemoteAddr())
	peerInfo := &PeerInfo{
		Ip:     ip,
		Port:   port,
		Conn:   conn,
		Key:    key,
		PeerId: make([]byte, 20),
	}

	// h := sha1.Sum([]byte(conn.RemoteAddr().String()))
	// peerInfo.Key = binary.BigEndian.Uint64(h[:])

	// send handshake
	sh := &HandShakeParams{
		PStrLen:  uint8(len(PROTOCOL)),
		Pstr:     PROTOCOL,
		Reserved: reserved,
		InfoHash: infoHash,
		PeerId:   clientId,
	}
	err = peerInfo.SendHandshake(sh)
	if err != nil {
		return nil, fmt.Errorf("error in sending handshake %s", err)
	}

	// verify handshake
	rh, err := peerInfo.RecvHandshake()
	if err != nil {
		return nil, err
	}
	if rh.Pstr != PROTOCOL {
		return nil, fmt.Errorf("invalid protocol string %s", rh.Pstr)
	}
	if !bytes.Equal(rh.InfoHash[:], sh.InfoHash[:]) {
		return nil, fmt.Errorf("invalid info hash, sent %x, recv %x", sh.InfoHash, rh.InfoHash)
	}
	copy(peerInfo.PeerId, rh.PeerId)
	peerInfo.Reserved = rh.Reserved
	fmt.Printf("peerId   : %x\n", peerInfo.PeerId)
	fmt.Printf("reserved : %x\n", rh.Reserved)

	return peerInfo, nil
}

type Peer struct {
	conn         net.Conn
	peerInfo     *PeerInfo
	torrentInfo  *TorrentInfo
	pieceManager *PieceManager

	// peer state
	am_choking      bool // this client is choking the peer
	am_interested   bool // this client is intereted in peer
	peer_choking    bool // peer is choking this client
	peer_interested bool // peer is interested in this client

	// extensions
	supports_ltep       bool
	supports_fast_peers bool
	supports_dht        bool

	bitmap *BitMap
	extMap map[string]uint8 // extension id's local to this peer

	// peer stats
	keepalive  int64
	downloaded int64 // bytes downloaded from this peer
	uploaded   int64 // bytes uploaded to this peer

	// pieces to get from peer
	pieceQueue []int        // list of piece indices
	pieceMap   map[int]bool // piece index -> presence in Q

	// external channels
	peerChannel chan *MessageParams

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
		peerInfo:     peerInfo,
		torrentInfo:  torrentInfo,
		pieceManager: pieceManager,

		am_choking:      true,
		am_interested:   false,
		peer_choking:    true,
		peer_interested: false,

		supports_ltep:       (peerInfo.Reserved[5]&0x10 > 1),
		supports_fast_peers: (peerInfo.Reserved[7]&0x05 > 1),
		supports_dht:        (peerInfo.Reserved[7]&0x01 > 1),

		bitmap: NewBitMap(pieceManager.bitmap.length),
		extMap: make(map[string]uint8),

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

func (p *Peer) Start(peerChannel chan *MessageParams) {
	p.peerChannel = peerChannel
	go p.start()
}

// peer state machine
func (p *Peer) start() {

	defer p.conn.Close()
	go p.handleRequestsToPeer()
	go p.handleRequestsFromPeer()

	go func() {
		// send extension handshake
		m := make(map[string]interface{})
		m["ut_metadata"] = 1

		payload := make(map[string]interface{})
		payload["m"] = m
		payload["v"] = "gotorrent 1.0"
		payload["reqq"] = 2000
		payload["metadata_size"] = 310

		enc, err := BEncode(payload)
		if err != nil {
			panic(err)
		}

		fmt.Println("payload : ", string(enc))
		p.toPeer <- &MessageParams{
			Type: MESSAGE_EXTENSION_PROTOCOL,
			Data: &MessageExtension{
				EType:   0x00,
				Payload: enc,
			},
		}

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

		// if there are messages to be sent to peer
		select {
		case v := <-p.toPeer:
			p.sendMessage(v)
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
		n = 4
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

	// fmt.Println("from peer <= ", p.msgIdMap[mtype])

	// handle bitfield message
	if mtype == MESSAGE_BITFIELD {
		if int(mlen-1) != len(p.bitmap.Bytes()) {
			return nil, fmt.Errorf("invalid number of bitmap bytes")
		}
		err := RecvNBytes(p.conn, p.bitmap.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error reading bitfield")
		}

		message.Data = &MessageBitField{
			BitField: p.bitmap.Bytes(),
		}
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
		return nil, fmt.Errorf("error reading message payload")
	}

	switch mtype {
	case MESSAGE_CHOKE:
		p.peer_choking = true

	case MESSAGE_UNCHOKE:
		p.peer_choking = false

	case MESSAGE_INTERESTED:
		p.peer_interested = true

	case MESSAGE_NOT_INTERESTED:
		p.peer_interested = false

	case MESSAGE_HAVE:
		pi := binary.BigEndian.Uint32(payload[0:4])
		p.bitmap.SetBit(int(pi))
		message.Data = &MessageHave{
			PieceIndex: pi,
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

	case MESSAGE_EXTENSION_PROTOCOL:
		mdata := &MessageExtension{
			EType:   payload[0],
			Payload: payload[1:],
		}
		p.parseExtensionMessage(mdata)
		message.Data = mdata

	default:
		fmt.Println("unrecognized message type ", message.Type)
	}

	return message, nil
}

func (p *Peer) parseExtensionMessage(msg *MessageExtension) {

	dec, err := BDecode(msg.Payload)
	if err != nil {
		panic(err)
	}
	payload := dec.(map[string]interface{})

	switch msg.EType {
	case 0x00:

		if v, ok := payload["m"]; ok {
			m := v.(map[string]interface{})
			for k, v := range m {
				id := v.(int64)
				fmt.Println(k, v)
				p.extMap[k] = uint8(id)
			}
			// fmt.Println()
		}

		// if v, ok := payload["complete_ago"]; ok {
		// 	val := v.(int64)
		// 	fmt.Println("complete_ago : ", val)
		// }

		// if v, ok := payload["reqq"]; ok {
		// 	val := v.(int64)
		// 	fmt.Println("reqq : ", val)
		// }

		// if v, ok := payload["upload_only"]; ok {
		// 	val := v.(int64)
		// 	fmt.Println("upload_only : ", val)
		// }

	default:
		fmt.Printf("unrecognised extension %x\n", msg.EType)
	}
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

	case MESSAGE_EXTENSION_PROTOCOL:
		v := message.Data.(*MessageExtension)
		payload = append(payload, v.EType)
		payload = append(payload, v.Payload...)

	default:
		fmt.Println("unrecognized message : ", message.Type)
		return nil
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

// get metadata from the peer
func (p *Peer) GetMetadata() []byte {
	return nil
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
