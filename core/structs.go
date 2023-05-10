package core

// magnet link information
type MagnetInfo struct {
	ExactTopic   string
	DisplayName  string
	ExactLength  int64
	InfoHash     []byte
	AnnounceList []string
}

type FileInfo struct {
	Length int64  // "length" : Length of the file
	Path   string // "path" : path of the file
	Md5sum string // "md5sum" : (optional)
}

// metainfo, .torrent file
type TorrentInfo struct {
	Announce     string   // "announce" : tracker url
	AnnounceList []string // "annouce-list" : (optional) tracker urls
	CreationDate int64    // "creation date": (optional) unix epoch time of torrent creation
	Comment      string   // "comment": (optional) text comment by author
	CreatedBy    string   // "created by" : (optional) name and version of the program
	Encoding     string   // "encoding": (optional)
	PieceLength  int64    // "peice length" :length of each piece
	PieceHashes  []byte   // "pieces" : concatenated list of <20byte-sha1-checksum> of pieces
	IsPrivate    bool     // "private" : (optional)
	Name         string   // name of the root directory
	Length       int64    // length of the complete file
	Md5sum       string
	Files        []*FileInfo // list of files, works for both single and multi file structure

	// custom fields
	MetaInfo         []byte // metadata bytes
	InfoHash         []byte // sha1 of info dictionary
	IsMultiFile      bool   // if the torrent has multiple files
	IsFromMagnetLink bool   // if the struct is created using magnet link
	NumPieces        int64  // number of pieces in the torrent
	LastPieceLength  int64  // last piece length
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
	Data   interface{}
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

type MessagePieceBlock struct {
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

type MessageExtension struct {
	EType   uint8 // extension type
	Payload []byte
}

type MessageDefault struct {
	Payload []byte
}

type TorrentMessage struct {
	Type int
	Data string
}

type PeerMessage struct {
	Key  string // peer identifier
	Type int
	Data interface{}
}

type TrackerMessage struct {
	Key  string // tracker identifier
	Type int
	Data interface{} // paylod
}

// http structs
type AnnounceReq struct {
	InfoHash   [20]byte // 20 bytes SHA1 hash
	PeerId     [20]byte // 20 byte client generated ID
	Port       uint16   // range 6881-6889
	Uploaded   int64    // total amount uploaded since 'started' event
	Downloaded int64    // total amount downloaded since 'started' event
	Left       int64    // the number of bytes that has to be downloaded
	Compact    int      // 0 or 1
	NoPeerId   int      // (Optional) tracker can omit peer id in peers
	Event      string   // 'started', 'completed', 'stopped'
	Ip         string   // (Optional)
	Numwant    int      // (Optional) number of peers client would like to recieve, default is 50
	Key        string   // (Optional) additional identification
	TrackerId  string   // (Optional)
}

type AnnounceRes struct {
	Interval    int64 // The waiting time between requests
	MinInterval int64 // (Optional) the minimum announce interval
	Complete    int64 // number of peers with the entire file
	Incomplete  int64 // number of non-seeder peers (leechers)
	TrackerId   string
	ExternalIP  [4]byte
	Peers       []*PeerInfo // ipv4
	PeersV6     []*PeerInfo // ipv6
}

// udp structs
type ConnectReqUDP struct {
	ProtocolID    uint64 // 0x41727101980, magic constant
	Action        uint32 // 0 for connect
	TransactionId uint32
}

type ConnectResUDP struct {
	Action        uint32 // 0 for connect
	TransactionId uint32
	ConnectionId  uint64
}

type AnnounceReqUDP struct {
	ConnectionId  uint64
	Action        uint32 // 1 for announce
	TransactionId uint32
	InfoHash      [20]byte
	PeerId        [20]byte
	Downloaded    uint64
	Left          uint64
	Uploaded      uint64
	Event         uint32 // 0: none; 1: Completed, 2: started, 3: stopped
	IpAddr        uint32 // 0 default
	Key           uint32
	NumWant       int32 // -1 default
	Port          uint16
}

type AnnounceResUDP struct {
	Action        uint32
	TransactionId uint32
	Interval      uint32
	Leechers      uint32
	Seeders       uint32
	Peers         []*PeerInfo
}
