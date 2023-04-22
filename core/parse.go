package core

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// parse .torrent file
func ParseTorrentFile(tpath string) (*TorrentInfo, error) {

	bytes, err := os.ReadFile(tpath)
	if err != nil {
		panic(err)
	}

	bytes = []byte(strings.TrimSpace(string(bytes)))
	dec, err := BDecode(bytes)
	if err != nil {
		panic(err)
	}

	dmap := dec.(map[string]interface{})
	tinfo := &TorrentInfo{}

	for k, v := range dmap {

		switch k {
		case "announce":
			tinfo.Announce = v.(string)

		case "announce-list":
			tinfo.AnnounceList = make([]string, 0)
			val := v.([]interface{})
			for _, e := range val {
				entries := e.([]interface{})
				for _, entry := range entries {
					tinfo.AnnounceList = append(tinfo.AnnounceList, entry.(string))
				}

			}

		case "creation date":
			tinfo.CreationDate = v.(int)

		case "comment":
			tinfo.Comment = v.(string)

		case "created by":
			tinfo.CreatedBy = v.(string)

		case "encoding":
			tinfo.Encoding = v.(string)

		case "info":
			info := v.(map[string]interface{})
			ibytes, err := BEncode(info)
			if err != nil {
				return nil, err
			}
			tinfo.InfoHash = make([]byte, 20)
			infoHash := sha1.Sum(ibytes)
			copy(tinfo.InfoHash, infoHash[:])

			err = parseFileInfo(tinfo, info)
			if err != nil {
				return nil, err
			}
		}
	}

	// generated fields
	tinfo.NumPieces = uint64(len(tinfo.PieceHashes) / 20)
	fsize := uint64(0)
	for _, f := range tinfo.Files {
		fsize += uint64(f.Length)
	}
	tinfo.FileSize = fsize

	return tinfo, nil
}

// populate torrentinfo
func parseFileInfo(tinfo *TorrentInfo, info map[string]interface{}) error {

	// parse common fields
	for k, v := range info {
		switch k {
		case "piece length":
			tinfo.PieceLength = v.(int)

		case "pieces":
			pbytes := []byte(v.(string))
			tinfo.PieceHashes = pbytes

		case "private":
			tinfo.Private = v.(int)

		case "name":
			tinfo.Name = v.(string)

		}
	}

	tinfo.Files = make([]*FileInfo, 0)

	// handle multi file mode
	v, ok := info["files"]
	if ok {
		flist := v.([]interface{})
		for _, f := range flist {
			imap := f.(map[string]interface{})
			fileInfo := &FileInfo{}
			for k, v := range imap {
				switch k {
				case "length":
					fileInfo.Length = v.(int)
				case "path":
					fileInfo.Path = v.(string)
				case "md5sum":
					fileInfo.Md5sum = v.(string)
				}
			}
			tinfo.Files = append(tinfo.Files, fileInfo)
		}

		return nil
	}

	// handle single file mode
	fileInfo := &FileInfo{
		Path: tinfo.Name,
	}
	v, ok = info["length"]
	if ok {
		fileInfo.Length = v.(int)
	}
	v, ok = info["md5sum"]
	if ok {
		fileInfo.Md5sum = v.(string)
	}

	tinfo.Files = append(tinfo.Files, fileInfo)
	return nil
}

// parse magnet link
func ParseMagnetLink(mlink string) (*MagnetInfo, error) {

	mlink = strings.TrimSpace(mlink)
	u, err := url.Parse(mlink)
	if err != nil {
		return nil, err
	}

	params, _ := url.ParseQuery(u.RawQuery)
	xt := params["xt"][0]
	dn := params["dn"][0]
	xl := params["xl"][0]
	xli, _ := strconv.Atoi(xl)
	hash := strings.Split(xt, ":")[2]
	tlist := params["tr"]

	if len(hash) == 32 {
		dec, _ := base32.StdEncoding.DecodeString(hash)
		hash = strings.ToUpper(hex.EncodeToString(dec))
	}

	hbytes, _ := hex.DecodeString(hash)

	minfo := &MagnetInfo{
		ExactTopic:   xt,
		DisplayName:  dn,
		ExactLength:  xli,
		InfoHash:     hbytes,
		AnnounceList: tlist,
	}

	return minfo, nil
}
