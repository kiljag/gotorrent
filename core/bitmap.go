package core

type BitMap struct {
	length int
	bitmap []byte
}

func NewBitMap(len int) *BitMap {
	bl := len / 8
	if len%8 != 0 {
		bl++
	}

	return &BitMap{
		length: len,
		bitmap: make([]byte, bl),
	}
}

// return number of bits in the bitmap
func (b *BitMap) Length() int {
	return b.length
}

// return bytes stored in the bitmap
func (b *BitMap) Bytes() []byte {
	return b.bitmap
}

// check if i'th bit is set in the bitmap
func (b *BitMap) IsSet(i int) bool {
	if i >= b.length {
		return false
	}
	bi, off := i/8, i%8
	t := b.bitmap[bi] >> (7 - off) & 0x01
	return t == 0x01
}

// set i'th bit in the bitmap
func (b *BitMap) SetBit(i int) {
	if i >= b.length {
		return
	}
	bi, off := i/8, i%8
	t := b.bitmap[bi] | (0x01 << (7 - off))
	b.bitmap[bi] = t
}

// unset i'th bit in the bitmap
func (b *BitMap) UnsetBit(i int) {
	if i >= b.length {
		return
	}
	bi, off := i/8, i%8
	t := b.bitmap[bi] & ^(0x01 << (7 - off))
	b.bitmap[bi] = t
}
