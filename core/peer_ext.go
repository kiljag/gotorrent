package core

import (
	"fmt"
	"time"
)

const (
	// extensions
	EXT_PEX             = "ut_pex"
	EXT_METADATA        = "ut_metadata"
	EXT_UPLOAD_ONLY     = "upload_only"
	EXT_HOLEPUNCH       = "ut_holepunch"
	EXT_PAYMENT_ADDRESS = "ut_payment_address"
	EXT_DONTHAVE        = "lt_donthave"
)

func (p *Peer) handleExtensionMessage(msg *MessageExtension) {

	dec, err := BDecode(msg.Payload)
	if err != nil {
		panic(err)
	}
	payload := dec.(map[string]interface{})

	// handle handshake
	if msg.EType == 0x00 {
		for k, v := range payload {
			if k == "m" {
				m := v.(map[string]interface{})
				for ext, id := range m {
					p.extensionMap[ext] = uint8(id.(int64))
				}
			} else {
				p.extensionProps[k] = v
			}
		}
		// fmt.Println(p.key, "emap : ", p.extensionMap)
		// fmt.Println(p.key, "eprops : ", p.extensionProps)
		return
	}

	// metadata
	if p.extensionMap[EXT_METADATA] > 0 &&
		p.extensionMap[EXT_METADATA] == msg.EType {

		fmt.Println(p.key, "recieved metadata")
		if ch, ok := p.extensionChannels[EXT_METADATA]; ok {
			ch <- msg
		}
		return
	}

}

func (p *Peer) SupportsLTEP() bool {
	return (p.peerInfo.Reserved[5] & 0x10) != 0x00
}

func (p *Peer) SupportExtFastPeers() bool {
	return (p.peerInfo.Reserved[7] & 0x04) != 0x00
}

func (p *Peer) SupportsExtDHT() bool {
	return (p.peerInfo.Reserved[7] & 0x01) != 0x00
}

func (p *Peer) SupportsExtMetadata() bool {
	if v, ok := p.extensionMap[EXT_METADATA]; ok {
		return v > 0
	}
	return false
}

// schedule to fetch metadata from peer
func (p *Peer) ScheduleMetaInfoFetch() {
	if len(p.extensionMap) == 0 {
		time.Sleep(2 * time.Second)
	}

	if !p.SupportsExtMetadata() {
		fmt.Println(p.key, "metadata extension not supported")
		return
	}

	msize := 0
	if v, ok := p.extensionProps["metadata_size"]; ok {
		msize = int(v.(int64))
	}
	if msize == 0 {
		return
	}

	go p.fetchMetadata(msize)
}

func (p *Peer) fetchMetadata(msize int) {

	fmt.Println(p.key, "fetching metadata")

	blockSize := (16 * 1024)
	numBlocks := msize / blockSize
	if msize%blockSize != 0 {
		numBlocks++
	}

	blockRequests := make([]string, 0)
	for i := 0; i < numBlocks; i++ {
		rmap := make(map[string]interface{})
		rmap["msg_type"] = 0
		rmap["piece"] = int64(i)
		req, err := BEncode(rmap)
		if err != nil {
			fmt.Println(p.key, err)
		}
		blockRequests = append(blockRequests, string(req))
	}

	// metaInfo := make([]byte, msize)
	metaChannel := make(chan interface{})
	p.extensionChannels[EXT_METADATA] = metaChannel

	// send requests one by one
	for _, req := range blockRequests {
		fmt.Println(p.key, "req : ", req)

		p.toPeer <- &MessageParams{
			Type: MESSAGE_LTEP,
			Data: &MessageExtension{
				EType:   p.extensionMap[EXT_METADATA],
				Payload: []byte(req),
			},
		}

		v := <-metaChannel
		fmt.Printf("got response %+v", v)
		msg := v.(*MessageExtension)
		dec, n, err := bDecodeDict(msg.Payload)
		if err != nil {
			fmt.Println(p.key, err)
		}
		fmt.Println(p.key, "metadata n : ", n)
		if v, ok := dec["msg_type"]; ok {
			if v.(int64) == 2 { // type reject
				fmt.Println(p.key, "got reject message")
				return
			}
		}
	}
}
