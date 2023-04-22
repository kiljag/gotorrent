package core

import (
	"testing"
)

func TestBEncodeInt(t *testing.T) {
	got := string(bEncodeInt(34))
	want := "i34e"
	if got != want {
		t.Errorf("bencode int : got %s, want %s", got, want)
	}
}

func TestBEncodeStr(t *testing.T) {
	got := string(bEncodeStr("hello"))
	want := "5:hello"
	if got != want {
		t.Errorf("bencode str : got %s, want %s", got, want)
	}
}

func TestBEncodeList(t *testing.T) {
	l := []interface{}{"hello", "world"}
	enc, _ := bEncodeList(l)
	got := string(enc)
	want := "l5:hello5:worlde"
	if got != want {
		t.Errorf("bencode list : got %s, want %s", got, want)
	}
}

func TestBEncodeDict(t *testing.T) {
	d := make(map[string]interface{})
	d["hello"] = "world"
	d["apples"] = 4
	enc, _ := bEncodeDict(d)
	got := string(enc)
	want := "d6:applesi4e5:hello5:worlde"
	if got != want {
		t.Errorf("bencode dict : got %s, want %s", got, want)
	}
}

func TestBDecodeInt(t *testing.T) {
	dec, dl, _ := bDecodeInt([]byte("i31e"))
	wdec := 31
	wlen := 4
	if dec != wdec {
		t.Errorf("bdecode int : got %d, wanted %d", dec, wdec)
	}
	if dl != wlen {
		t.Errorf("bdecode int : got %d, wanted %d", dl, wlen)
	}
}

func TestBDecodeStr(t *testing.T) {
	dec, len, _ := bDecodeStr([]byte("5:hello"))
	wdec := "hello"
	wlen := 7
	if dec != wdec {
		t.Errorf("bdecode str, dec : got %s, wanted %s", dec, wdec)
	}
	if len != wlen {
		t.Errorf("bdecode str, dl : got %d, wanted %d", len, wlen)
	}
}

func TestBDecodeList(t *testing.T) {
	// input 1
	dec, dl, _ := bDecodeList([]byte("le"))
	wll, wdl := 0, 2
	if len(dec) != wll {
		t.Errorf("len(list), got %d, wanted %d", len(dec), wll)
	}
	if dl != wdl {
		t.Errorf("dlen : got %d, wanted %d", dl, wdl)
	}

	// input 2
	dec, dl, _ = bDecodeList([]byte("l5:hello3:cowe"))
	wll, wdl = 2, 14
	if len(dec) != wll {
		t.Errorf("len(list), got %d, wanted %d", len(dec), wll)
	}
	if dl != wdl {
		t.Errorf("dlen : got %d, wanted %d", dl, wdl)
	}

	// input 3
	dec, dl, _ = bDecodeList([]byte("ll5:helloel3:cowee"))
	wll, wdl = 2, 18
	if len(dec) != wll {
		t.Errorf("len(list), got %d, wanted %d", len(dec), wll)
	}
	if dl != wdl {
		t.Errorf("dlen : got %d, wanted %d", dl, wdl)
	}
}

func TestBDecodeDict(t *testing.T) {
	// input 1
	dec, dl, _ := bDecodeDict([]byte("de"))
	wl, wdl := 0, 2
	if len(dec) != wl {
		t.Errorf("len(map), got %d, wanted %d", len(dec), wl)
	}
	if dl != wdl {
		t.Errorf("dlen : got %d, wanted %d", dl, wdl)
	}

	// input 1
	dec, dl, _ = bDecodeDict([]byte("d3:cow3:moo4:spam4:eggse"))
	wl, wdl = 2, 24
	if len(dec) != wl {
		t.Errorf("len(map), got %d, wanted %d", len(dec), wl)
	}
	if dl != wdl {
		t.Errorf("dlen : got %d, wanted %d", dl, wdl)
	}
}
