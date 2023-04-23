package core

import (
	"fmt"
	"os"
	"time"
)

const (
	PIECE_NOT_STARTED = 0
	PIECE_IN_PROGRESS = 1
	PIECE_COMPLETED   = 2

	//
	PIECE_BLOCK_LENGTH = 16 * 1024 // 16KB
)

// piece structure
type Piece struct {
	index  int
	length int
	data   []byte

	// blocks
	blocks          []uint32          // list of 16KB offsets
	completedBlocks map[uint32]uint32 // offset -> block size

	// stats
	status         int   // current status of the piece
	creationTime   int64 // for rescheduling purpose
	scheduledTime  int64 //
	completionTime int64 //
}

// manages piece creation and writing to file
type PieceManager struct {
	torrentInfo *TorrentInfo

	// piece
	bitField     []byte
	pieceCache   []*Piece       // cache to reduce make([]byte) calls
	pieceMap     map[int]*Piece // peding pieces
	pieceChannel chan int       // to pass downloaded piece indices
	dummy        *Piece         // dummy piece

	downloaded int // number of downloaded pieces

	// io
	fdMap map[string]*os.File // path to fd
}

func NewPieceManager(torrentInfo *TorrentInfo) *PieceManager {

	// bitfield
	bl := torrentInfo.NumPieces / 8
	if torrentInfo.NumPieces%8 != 0 {
		bl++
	}
	bitField := make([]byte, bl)

	// initialize pieceCache
	numPieces := 50
	if numPieces > int(torrentInfo.NumPieces) {
		numPieces = int(torrentInfo.NumPieces)
	}
	pieceCache := make([]*Piece, 0)
	for i := 0; i < numPieces; i++ {
		pieceCache = append(pieceCache, &Piece{
			data: make([]byte, torrentInfo.PieceLength),
		})
	}

	// dummy
	dummy := &Piece{
		data: make([]byte, torrentInfo.PieceLength),
	}

	return &PieceManager{
		torrentInfo: torrentInfo,

		bitField:     bitField,
		pieceCache:   pieceCache,
		pieceMap:     make(map[int]*Piece, 0),
		pieceChannel: make(chan int),
		dummy:        dummy,

		downloaded: 0,

		fdMap: make(map[string]*os.File),
	}
}

func (pm *PieceManager) start() {
	go pm.startPieceManager()
}

// piece manager
func (pm *PieceManager) startPieceManager() {
	go pm.handleDownloadedPieces()
}

// returns a struct representing a piece
func (pm *PieceManager) getPiece(pi int) *Piece {

	// return if the piece exists
	if piece, ok := pm.pieceMap[pi]; ok {
		return piece
	}

	plen := pm.torrentInfo.PieceLength
	off := pi * pm.torrentInfo.PieceLength
	if (pm.torrentInfo.FileSize - uint64(off)) < uint64(plen) {
		plen = int(pm.torrentInfo.FileSize) - off
	}

	// if it is last piece or piece cache is empty
	var piece *Piece
	if plen != pm.torrentInfo.PieceLength || len(pm.pieceCache) == 0 {
		piece = &Piece{
			data: make([]byte, plen),
		}
	} else {
		// TODO : use mutex
		piece = pm.pieceCache[0]
		pm.pieceCache = pm.pieceCache[1:]
	}

	piece.index = pi
	piece.length = plen
	piece.status = PIECE_NOT_STARTED
	piece.creationTime = time.Now().Unix()

	// blocks
	piece.completedBlocks = make(map[uint32]uint32)
	piece.blocks = make([]uint32, 0)
	for off := 0; off < piece.length; off += PIECE_BLOCK_LENGTH {
		piece.blocks = append(piece.blocks, uint32(off))
	}

	pm.pieceMap[pi] = piece
	return piece
}

// handle the piece that is downloaded and verified
func (pm *PieceManager) completePiece(pi int) {
	pm.pieceChannel <- pi
	// close the channel if complete file is downloaded.
	pm.downloaded++
	if pm.downloaded == int(pm.torrentInfo.NumPieces) {
		close(pm.pieceChannel)
	}
}

// Goroutine : handle downloaded and verified pieces
func (pm *PieceManager) handleDownloadedPieces() {
	for {

		pi, ok := <-pm.pieceChannel
		if !ok {
			fmt.Println("piece channel closed")
			break
		}

		// save piece
		piece := pm.pieceMap[pi]
		pieceData := piece.data
		pieceBegin := pi * pm.torrentInfo.PieceLength
		pieceEnd := pieceBegin + piece.length
		for _, fileInfo := range pm.torrentInfo.Files {
			if pieceBegin >= pieceEnd {
				break
			}
			if fileInfo.End <= pieceBegin {
				continue
			}
			writeLen := pieceEnd - pieceBegin
			if fileInfo.End-pieceBegin < writeLen {
				writeLen = fileInfo.End - pieceBegin
			}
			fmt.Printf("fileBegin : %d, fileEnd : %d\n", fileInfo.Begin, fileInfo.End)
			fmt.Printf("pieceBegin : %d, pieceEnd : %d, writeLen : %d\n", pieceBegin, pieceEnd, writeLen)

			fileOffset := int64(pieceBegin) - int64(fileInfo.Begin)
			writeData := pieceData[:writeLen]
			pieceData = pieceData[writeLen:]
			pieceBegin += writeLen

			// open fd
			filePath := pm.torrentInfo.Files[0].Path
			if _, ok := pm.fdMap[filePath]; !ok {
				fmt.Println("opening fd for ", filePath)
				fd, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					panic(err)
				}
				pm.fdMap[filePath] = fd
			}
			fd := pm.fdMap[filePath]
			fd.Seek(fileOffset, 0)
			fd.Write(writeData)

			// add piece to cache //TODO : use mutex
			if piece.length == pm.torrentInfo.PieceLength {
				pm.pieceCache = append(pm.pieceCache, piece)
			}
		}
	}

	fmt.Println("end of piece handler")
}
