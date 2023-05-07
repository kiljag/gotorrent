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

// custom message types
type MessagePieceCompleted struct {
	PieceIndex uint32
}

type MessagePieceCancelled struct {
	PieceIndex uint32
}
