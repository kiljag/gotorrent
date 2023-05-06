package core

import (
	"crypto/rand"
	"crypto/sha1"
	"net"
)

func GeneratePeerId() []byte {
	id := make([]byte, 20)
	copy(id[:8], []byte("-GT0101-"))
	buf := make([]byte, 128)
	rand.Read(buf)
	hb := sha1.Sum(buf)
	copy(id[8:], hb[8:])
	return id
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
