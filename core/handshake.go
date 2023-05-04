package core

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

const (
	PROTOCOL = "BitTorrent protocol"
)

// start handshake with peer
func StartHandshake(peerInfo *PeerInfo, infoHash []byte, clientId []byte) error {

	// start tcp connection with remote peer
	addr := fmt.Sprintf("%s:%d", peerInfo.Ip, peerInfo.Port)

	// p.conn, err := net.Dial("tcp", addr)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("connected to peer %s:%d\n", peerInfo.Ip, peerInfo.Port)

	// intiate handshake
	sh := &HandShakeParams{
		PStrLen:  uint8(len(PROTOCOL)),
		Pstr:     PROTOCOL,
		Reserved: make([]byte, 8),
		InfoHash: make([]byte, 20),
		PeerId:   make([]byte, 20),
	}

	fmt.Printf("infoHash : %x\n", infoHash)
	fmt.Printf("clientId : %x\n", clientId)
	copy(sh.InfoHash, infoHash)
	copy(sh.PeerId, clientId)

	err = sendHandShake(conn, sh)
	if err != nil {
		return err
	}

	rh, err := recvHandShake(conn)
	if err != nil {
		return err
	}

	// verify handshake
	if rh.Pstr != PROTOCOL {
		return fmt.Errorf("invalid protocol string %s", rh.Pstr)
	}
	if !bytes.Equal(rh.InfoHash[:], sh.InfoHash[:]) {
		return fmt.Errorf("invalid info hash, sent %x, recv %x", sh.InfoHash, rh.InfoHash)
	}

	// save peerId
	peerId := make([]byte, 20)
	copy(peerId, rh.PeerId)

	fmt.Println("successful handshake")
	peerInfo.Conn = conn
	peerInfo.PeerId = peerId

	return nil
}

// recv and send handshake, returns (peerId, error)
func VerifyHandshake(conn net.Conn, clientId []byte) ([]byte, error) {

	rh, err := recvHandShake(conn)
	if err != nil {
		return nil, err
	}

	if rh.Pstr != PROTOCOL {
		return nil, fmt.Errorf("invalid protocol string %s", rh.Pstr)
	}

	// TODO : see if we have the file with infoHash

	peerId := make([]byte, 20)
	copy(peerId, rh.PeerId)
	sh := &HandShakeParams{
		PStrLen:  uint8(len(PROTOCOL)),
		Pstr:     PROTOCOL,
		Reserved: make([]byte, 8),
		InfoHash: rh.InfoHash,
		PeerId:   clientId,
	}
	err = sendHandShake(conn, sh)
	if err != nil {
		return nil, err
	}

	return peerId, nil
}

func sendHandShake(conn net.Conn, h *HandShakeParams) error {
	payload := make([]byte, 0)
	payload = append(payload, h.PStrLen)
	payload = append(payload, []byte(h.Pstr)...)
	payload = append(payload, h.Reserved[:]...)
	payload = append(payload, h.InfoHash[:]...)
	payload = append(payload, h.PeerId[:]...)
	return SendNBytes(conn, payload)
}

// read handshake response
func recvHandShake(conn net.Conn) (*HandShakeParams, error) {

	buf := make([]byte, 68)
	err := RecvNBytes(conn, buf)
	if err != nil {
		return nil, err
	}
	// read protocol string
	h := &HandShakeParams{
		Reserved: make([]byte, 8),
		InfoHash: make([]byte, 20),
		PeerId:   make([]byte, 20),
	}
	h.PStrLen = buf[0]
	h.Pstr = string(buf[1:20])
	offset := 20

	// read reserved bytes
	copy(h.Reserved[:], buf[offset:offset+8])
	offset += 8
	// read info hash
	copy(h.InfoHash[:], buf[offset:offset+20])
	offset += 20
	// read peerId
	copy(h.PeerId[:], buf[offset:offset+20])
	offset += 20
	return h, nil
}
