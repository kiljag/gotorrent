package core

import "time"

const (
	PIECE_NOT_STARTED = 0
	PIECE_IN_PROGRESS = 1
	PIECE_COMPLETED   = 2
)

// complete piece
type Piece struct {
	Index          int
	Length         int
	Data           []byte
	Status         int    // current status of the piece
	CreationTime   int64  // for rescheduling purpose
	ScheduledTime  int64  //
	CompletionTime int64  //
	PeerKey        uint64 // key of source peer
}

func NewPiece(pi, plen int) *Piece {
	return &Piece{
		Index:        pi,
		Length:       plen,
		Data:         make([]byte, plen),
		Status:       PIECE_NOT_STARTED,
		CreationTime: time.Now().Unix(),
	}
}
