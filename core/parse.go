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
			tinfo.CreationDate = v.(int64)

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
			parseFileInfo(tinfo, info)
		}
	}

	// generated fields
	tinfo.NumPieces = int64(len(tinfo.PieceHashes) / 20)
	tinfo.Length = 0
	for _, f := range tinfo.Files {
		tinfo.Length += int64(f.Length)
	}
	tinfo.LastPieceLength = tinfo.Length - (tinfo.NumPieces-1)*tinfo.PieceLength

	return tinfo, nil
}

// populate torrentinfo, will throw a panic in case of error
func parseFileInfo(tinfo *TorrentInfo, info map[string]interface{}) {

	// parse common fields
	if v, ok := info["piece length"]; ok {
		tinfo.PieceLength = v.(int64)
	}

	if v, ok := info["pieces"]; ok {
		pbytes := []byte(v.(string))
		tinfo.PieceHashes = pbytes
	}

	if v, ok := info["private"]; ok {
		tinfo.IsPrivate = (v.(int64) == 1)
	}

	if v, ok := info["name"]; ok {
		tinfo.Name = v.(string)
	}

	if v, ok := info["length"]; ok {
		tinfo.Length = v.(int64)
	}

	if v, ok := info["md5sum"]; ok {
		tinfo.Md5sum = v.(string)
	}

	tinfo.Files = make([]*FileInfo, 0)

	// multi file torrent
	if v, ok := info["files"]; ok {
		tinfo.IsMultiFile = true
		flist := v.([]interface{})
		for _, f := range flist {
			imap := f.(map[string]interface{})
			fileInfo := &FileInfo{}
			for k, v := range imap {
				switch k {
				case "length":
					fileInfo.Length = v.(int64)
				case "path":
					plist := v.([]interface{})
					elist := make([]string, 0)
					for _, e := range plist {
						elist = append(elist, e.(string))
					}
					fileInfo.Path = strings.Join(elist, "/")
				case "md5sum":
					fileInfo.Md5sum = v.(string)
				}
			}
			tinfo.Files = append(tinfo.Files, fileInfo)
		}
	}

	// handle single file mode
	if !tinfo.IsMultiFile {
		tinfo.Files = append(tinfo.Files, &FileInfo{
			Length: tinfo.Length,
			Path:   tinfo.Name,
			Md5sum: tinfo.Md5sum,
		})
	}
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
		ExactLength:  int64(xli),
		InfoHash:     hbytes,
		AnnounceList: tlist,
	}

	return minfo, nil
}
