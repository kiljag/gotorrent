package core

import (
	"encoding/hex"
	"testing"
)

// test bitmap operations

func TestBitMap(t *testing.T) {

	blen := 45
	rbytes := []byte{0x03, 0x03, 0xa1, 0x09, 0x03, 0xf8}
	// hex : 0303a10903f8

	setList := []int{1, 0}
	indices := []int{10, 18}
	states := []string{
		"0323a10903f8",
		"0323810903f8",
	}

	bitmap := NewBitMap(blen)
	copy(bitmap.Bytes(), rbytes)

	for i, set := range setList {
		if set == 1 {
			bitmap.SetBit(indices[i])
		} else {
			bitmap.UnsetBit(indices[i])
		}

		h := hex.EncodeToString(bitmap.Bytes())

		if h != states[i] {
			t.Errorf("bitmap, i : got %s, wanted %s", h, states[i])
		}
	}
}
