package core

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// test piece manager

func TestPieceManager(t *testing.T) {

	randomBytes := make([]byte, 0)
	fileInfos := make([]*FileInfo, 0)

	for i := 0; i < 5; i++ {
		flen := (1500 + rand.Intn(500)) * 1024
		rbytes := make([]byte, flen)
		crand.Read(rbytes)
		randomBytes = append(randomBytes, rbytes...)
		fileName := fmt.Sprintf("%d.bin", i)
		fileInfos = append(fileInfos, &FileInfo{
			Length: int64(flen),
			Path:   fileName,
		})
	}

	tinfo := &TorrentInfo{}
	tinfo.Files = fileInfos
	tinfo.Length = 0
	for _, f := range fileInfos {
		tinfo.Length += f.Length
	}
	tinfo.PieceLength = 256 * 1024 // 256KB
	tinfo.NumPieces = tinfo.Length / tinfo.PieceLength
	if tinfo.Length%tinfo.PieceLength != 0 {
		tinfo.NumPieces++
	}
	tinfo.LastPieceLength = tinfo.Length - (tinfo.NumPieces-1)*tinfo.PieceLength

	// fmt.Println("file length : ", tinfo.Length)
	// fmt.Println("fbytes len :", len(randomBytes))
	// fmt.Println("num pieces : ", tinfo.NumPieces)
	// fmt.Println("piece len : ", tinfo.PieceLength)
	// fmt.Println("last piece len : ", tinfo.LastPieceLength)

	pieceIndices := make([]int, 0)
	for i := 0; i < int(tinfo.NumPieces); i++ {
		pieceIndices = append(pieceIndices, i)
	}

	// shuffle indices
	for i := 0; i < int(tinfo.NumPieces); i++ {
		ri := rand.Intn(len(pieceIndices))
		t := pieceIndices[i]
		pieceIndices[i] = pieceIndices[ri]
		pieceIndices[ri] = t
	}

	// fmt.Println("piece indices :", pieceIndices)

	root, err := os.MkdirTemp("", "*")
	if err != nil {
		panic(err)
	}
	// fmt.Println("root : ", root)
	defer os.RemoveAll(root)

	pm := NewPieceManager(tinfo)
	pm.setDownloadDir(root)
	pm.Start()
	for _, pi := range pieceIndices {
		piece := pm.getPiece(pi)
		begin := pi * int(tinfo.PieceLength)
		end := begin + piece.length

		copy(piece.data, randomBytes[begin:end])
		pm.savePiece(pi)
	}

	<-pm.doneSaving

	// read saved files
	savedBytes := make([]byte, 0)
	for _, f := range fileInfos {
		filePath := filepath.Join(pm.downloadDir, f.Path)
		b, err := os.ReadFile(filePath)
		if err != nil {
			panic(err)
		}
		savedBytes = append(savedBytes, b...)
	}

	if !bytes.Equal(randomBytes, savedBytes) {
		t.Errorf("error in piece wise saving")
	}

}
