package core

// magnet link information
type MagnetInfo struct {
	ExactTopic   string
	DisplayName  string
	ExactLength  int
	InfoHash     []byte
	AnnounceList []string
}

type FileInfo struct {
	Length int    // "length" : Length of the file
	Path   string // "path" : for multi file mode, same as Name for single file
	Md5sum string // "md5sum" : (optional)
}

// metainfo, .torrent file
type TorrentInfo struct {
	Announce     string      // "announce" : tracker url
	AnnounceList []string    // "annouce-list" : (optional) tracker urls
	CreationDate int         // "creation date": (optional) unix epoch time of torrent creation
	Comment      string      // "comment": (optional) text comment by author
	CreatedBy    string      // "created by" : (optional) name and version of the program
	Encoding     string      // "encoding": (optional)
	PieceLength  int         // "peice length" :length of each piece
	PieceHashes  []byte      // "pieces" : concatenated list of <20byte-sha1-checksum> of pieces
	Private      int         // "private" : (optional)
	Name         string      // "name" : filename or root directory
	Files        []*FileInfo // list of files, works for both single file and directory structure
	InfoHash     []byte      // sha1 of info dict

	// generated
	NumPieces uint64 // number of pieces in the file
	FileSize  uint64 // size of the entire file or directory
}

// tracker Announce request
type AnnounceReq struct {
	InfoHash   []byte // 20 bytes SHA1 hash
	PeerId     []byte // 20 byte client generated ID
	Port       int    // range 6881-6889
	Uploaded   int    // total amount uploaded since 'started' event
	Downloaded int    // total amount downloaded since 'started' event
	Left       int    // the number of bytes that has to be downloaded
	Compact    int    // 0 or 1
	NoPeerId   int    // (Optional) tracker can omit peer id in peers
	Event      string // 'started', 'completed', 'stopped'
	Ip         string // (Optional)
	Numwant    int    // (Optional) number of peers client would like to recieve, default is 50
	Key        string // (Optional) additional identification
	TrackerId  string // (Optional)
}

// tracker Announce response
type AnnounceRes struct {
	Interval    int // The waiting time between requests
	MinInterval int // (Optional) the minimum announce interval
	Complete    int // number of peers with the entire file
	Incomplete  int // number of non-seeder peers (leechers)
	TrackerId   string
	Peers       []byte // concatenated list of <4-byte-ip, 2-byte-port>
	PeersV6     []byte // ipv6 addresses
}

// peer hanshake response
// (68 bytes long)
type HandShakeParams struct {
	PStrLen  uint8
	Pstr     string
	Reserved []byte
	InfoHash []byte
	PeerId   []byte
}

// peer message
type MessageParams struct {
	Length uint32
	Type   uint8
	// all the extractable fields
	Data interface{}
}

type MessageHave struct {
	PieceIndex uint32
}

// different
type MessageBitField struct {
	BitField []byte
}

type MessageRequest struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

type MessagePiece struct {
	PieceIndex uint32
	Begin      uint32
	Block      []byte
}

type MessageCancel struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

type MessagePort struct {
	Port uint16
}
