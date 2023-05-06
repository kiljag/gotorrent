package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

const (
	PIECE_BLOCK_LENGTH   = 16 * 1024 // 16KB
	TORRENT_DOWNLOAD_DIR = "~/Downloads/"
)

// piece structure
type Piece struct {
	index  int
	length int
	data   []byte // len(data) should be length
}

// manages piece creation and writing to file
type PieceManager struct {
	torrentInfo      *TorrentInfo
	bitmap           *BitMap // bitmap of downloaded pieces
	downloadedPieces int     // number of downloaded pieces

	// pieces
	piecelock    sync.Mutex
	pieceCache   []*Piece       // cache to reduce make([]byte) calls
	pieceMap     map[int]*Piece // pieces which are in progress
	pieceChannel chan int       // to pass downloaded piece indices
	dummy        *Piece         // dummy piece
	doneSaving   chan bool      // pieces are saved

	// stats
	downloadedBytes int64
	uploadedBytes   int64

	// io
	downloadDir string
	writeFdMap  map[string]*os.File // path to fd
}

func NewPieceManager(torrentInfo *TorrentInfo) *PieceManager {
	homeDir, _ := os.UserHomeDir()
	dummy := &Piece{data: make([]byte, torrentInfo.Length)}

	return &PieceManager{
		torrentInfo:      torrentInfo,
		bitmap:           NewBitMap(int(torrentInfo.NumPieces)),
		downloadedPieces: 0,

		pieceCache:   make([]*Piece, 0),
		pieceMap:     make(map[int]*Piece, 0),
		pieceChannel: make(chan int),
		dummy:        dummy,
		doneSaving:   make(chan bool),

		downloadedBytes: 0,
		uploadedBytes:   0,

		downloadDir: filepath.Join(homeDir, "Downloads"),
		writeFdMap:  make(map[string]*os.File),
	}
}

func (pm *PieceManager) setDownloadDir(downloadDir string) {
	pm.downloadDir = downloadDir
}

func (pm *PieceManager) start() {
	go pm.handleDownloadedPieces()
}

// returns a struct representing a piece
func (pm *PieceManager) getPiece(pi int) *Piece {

	pm.piecelock.Lock()
	defer pm.piecelock.Unlock()

	// return if the piece exists
	if piece, ok := pm.pieceMap[pi]; ok {
		return piece
	}

	// if it is last piece or piece cache is empty
	var piece *Piece
	if len(pm.pieceCache) == 0 {
		piece = &Piece{
			data: make([]byte, pm.torrentInfo.PieceLength),
		}
	} else {
		piece = pm.pieceCache[0]
		pm.pieceCache = pm.pieceCache[1:]
	}

	// zero out bytes
	for i := 0; i < len(piece.data); i++ {
		piece.data[i] = 0x00
	}

	piece.index = pi
	piece.length = int(pm.torrentInfo.PieceLength)
	if pi == int(pm.torrentInfo.NumPieces)-1 {
		piece.length = int(pm.torrentInfo.LastPieceLength)
	}

	pm.pieceMap[pi] = piece
	return piece
}

// handle the piece that is downloaded and verified
func (pm *PieceManager) savePiece(pi int) {
	pm.pieceChannel <- pi
}

// get file descriptor to a file
func (pm *PieceManager) getFd(fpath string) *os.File {

	if _, ok := pm.writeFdMap[fpath]; !ok {

		filePath := filepath.Join(pm.downloadDir, fpath)
		os.MkdirAll(path.Dir(filePath), 0775)
		fmt.Println("opening file ", filePath)
		fd, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}

		pm.writeFdMap[fpath] = fd
	}
	return pm.writeFdMap[fpath]
}

// Goroutine : handle downloaded and verified pieces
func (pm *PieceManager) handleDownloadedPieces() {

	startTime := time.Now()

	for {
		pi, ok := <-pm.pieceChannel
		if !ok {
			fmt.Println("piece channel closed")
			break
		}
		// fmt.Println("saving piece ", pi)

		// save piece
		pm.piecelock.Lock()
		piece := pm.pieceMap[pi]
		pm.piecelock.Unlock()

		pieceBegin := pi * int(pm.torrentInfo.PieceLength)
		pieceEnd := pieceBegin + piece.length
		pieceOffset := 0

		// fmt.Printf("pieceBegin : %d, pieceEnd : %d\n", pieceBegin, pieceEnd)

		fileBegin := 0
		for _, fileInfo := range pm.torrentInfo.Files {
			fileEnd := fileBegin + int(fileInfo.Length)
			if fileEnd <= pieceBegin {
				fileBegin = fileEnd
				continue
			}
			writeLen := pieceEnd - pieceBegin
			if fileEnd-pieceBegin < writeLen {
				writeLen = fileEnd - pieceBegin
			}

			// fmt.Printf("fileBegin  : %d, fileEnd : %d, writeLen : %d\n", fileBegin, fileEnd, writeLen)
			fileOffset := pieceBegin - fileBegin
			fd := pm.getFd(fileInfo.Path)
			fd.Seek(int64(fileOffset), 0)
			_, err := fd.Write(piece.data[pieceOffset : pieceOffset+writeLen])
			if err != nil {
				panic(err)
			}

			fileBegin = fileEnd // begin offset of next file
			pieceBegin += writeLen
			pieceOffset += writeLen
			if pieceBegin >= pieceEnd {
				break
			}
		}

		// add piece to cache
		pm.downloadedPieces++
		pm.piecelock.Lock()
		pm.pieceCache = append(pm.pieceCache, piece)
		delete(pm.pieceMap, pi)
		pm.piecelock.Unlock()

		if pm.downloadedPieces == int(pm.torrentInfo.NumPieces) {
			break
		}
	}

	timeTaken := time.Since(startTime)
	fmt.Printf("downloaded %d pieces, took %s\n", pm.downloadedPieces, timeTaken)
	// close all write fds
	for _, fd := range pm.writeFdMap {
		fd.Close()
	}
	pm.doneSaving <- true
}
