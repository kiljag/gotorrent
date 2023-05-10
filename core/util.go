package core

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"net"
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
	msgIdMap[MESSAGE_LTEP] = "extension+ltep"
	msgIdMap[MESSAGE_HASH_REQUEST] = "hash+request"
	msgIdMap[MESSAGE_HASHES] = "hashes"
	msgIdMap[MESSAGE_HASH_REJECT] = "hash+reject"
	return msgIdMap
}

func GeneratePeerId() []byte {
	id := make([]byte, 20)
	copy(id[:8], []byte("-GT0101-"))
	buf := make([]byte, 128)
	rand.Read(buf)
	hb := sha1.Sum(buf)
	copy(id[8:], hb[8:])
	return id
}

func GenerateTransactionId() uint32 {
	var id [4]byte
	rand.Read(id[:])
	return binary.BigEndian.Uint32(id[:])
}

func GetAddrInfo(url string) (string, error) {
	ips, err := net.LookupIP(url)
	if err != nil {
		return "", err
	}
	return ips[0].String(), nil
}

// recv n complete bytes from tcp socket, n = len(buf)
func RecvNBytes(conn net.Conn, buf []byte) error {
	for len(buf) > 0 {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}

// send n complete bytes from tcp socket, n = len(buf)
func SendNBytes(conn net.Conn, buf []byte) error {
	for len(buf) > 0 {
		n, err := conn.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}
