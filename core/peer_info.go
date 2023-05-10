package core

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// peer info
type PeerInfo struct {
	Ip   net.IP
	Port uint16
	Key  string

	Conn     net.Conn
	IsActive bool // check if the peer is active or not
	PeerId   [20]byte
	Reserved [8]byte
}

func NewPeerInfo(ip net.IP, port uint16) *PeerInfo {

	if ip.String() == "127.0.0.1" {
		ip = net.IP([]byte{192, 168, 0, 102})
	}

	key := fmt.Sprintf("%s:%d", ip, port)
	key = fmt.Sprintf("(p)%21s", key)

	return &PeerInfo{
		Ip:   ip,
		Port: port,
		Key:  key,
	}
}

// open a tcp connection with peer, send and receive handshake
func (p *PeerInfo) SendVerifyHandshake(infoHash []byte, reserved []byte, clientId []byte) error {

	addr := fmt.Sprintf("%s:%d", p.Ip, p.Port)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("%s : peer connect error : %s", p.Key, err)
	}
	fmt.Printf("connected to peer %s -> %s\n", conn.LocalAddr(), conn.RemoteAddr())

	// send handshake
	sh := &HandShakeParams{
		PStrLen:  uint8(len(PROTOCOL)),
		Pstr:     PROTOCOL,
		Reserved: reserved[:],
		InfoHash: infoHash[:],
		PeerId:   clientId[:],
	}
	err = sendHandshake(conn, sh)
	if err != nil {
		return err
	}

	// verify handshake
	rh, err := recvHandshake(conn)
	if err != nil {
		return err
	}
	if rh.Pstr != PROTOCOL {
		return fmt.Errorf("%s : invalid protocol string %s", p.Key, rh.Pstr)
	}
	if !bytes.Equal(rh.InfoHash[:], sh.InfoHash[:]) {
		return fmt.Errorf("%s: invalid info hash, sent %x, recv %x", p.Key, sh.InfoHash, rh.InfoHash)
	}

	p.Conn = conn
	copy(p.PeerId[:], rh.PeerId)
	copy(p.Reserved[:], rh.Reserved)
	return nil
}

// recv, verify and send back handshake
func (p *PeerInfo) RecvVerifyHandshake(conn net.Conn, infoHash []byte, reserved []byte, clientId []byte) error {

	rh, err := recvHandshake(conn)
	if err != nil {
		return err
	}

	if rh.Pstr != PROTOCOL {
		return fmt.Errorf("invalid protocol string %s", rh.Pstr)
	}
	if !bytes.Equal(rh.InfoHash[:], infoHash) {
		return fmt.Errorf("mismatch in infoHash, recv %x, expected %x", rh.InfoHash, infoHash)
	}

	sh := &HandShakeParams{
		PStrLen:  uint8(len(PROTOCOL)),
		Pstr:     PROTOCOL,
		Reserved: reserved,
		InfoHash: infoHash,
		PeerId:   clientId,
	}

	err = sendHandshake(conn, sh)
	if err != nil {
		return err
	}

	p.Conn = conn
	copy(p.PeerId[:], rh.PeerId)
	copy(p.Reserved[:], rh.Reserved)
	return nil
}

func sendHandshake(conn net.Conn, h *HandShakeParams) error {
	payload := make([]byte, 0)
	payload = append(payload, h.PStrLen)
	payload = append(payload, []byte(h.Pstr)...)
	payload = append(payload, h.Reserved[:]...)
	payload = append(payload, h.InfoHash[:]...)
	payload = append(payload, h.PeerId[:]...)
	err := SendNBytes(conn, payload)
	if err != nil {
		return fmt.Errorf("%s handshake write error : %s", conn.RemoteAddr(), err)
	}
	return nil
}

// read handshake response
func recvHandshake(conn net.Conn) (*HandShakeParams, error) {
	buf := make([]byte, 68)
	err := RecvNBytes(conn, buf)
	if err != nil {
		return nil, fmt.Errorf("%s handshake read error : %s", conn.RemoteAddr(), err)
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
