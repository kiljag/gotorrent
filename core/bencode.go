package core

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

// bencoding and bdecoding primitives
/* 4 types can be encoded or decoded
int64
string or []byte
[]interface{}
[string]interface{}
*/

func BEncode(input interface{}) ([]byte, error) {

	switch v := input.(type) {
	case int, int32, int64:
		num := reflect.ValueOf(v).Int()
		return bEncodeInt(num), nil
	case string:
		return bEncodeStr(v), nil
	case []byte:
		return bEncodeStr(string(v)), nil
	case []interface{}:
		return bEncodeList(v)
	case map[string]interface{}:
		return bEncodeDict(v)
	default:
		return nil, fmt.Errorf("invalid type %s", v)
	}
}

func bEncodeInt(i int64) []byte {

	istr := fmt.Sprintf("%d", i)
	enc := make([]byte, 0)
	enc = append(enc, 'i')
	enc = append(enc, []byte(istr)...)
	enc = append(enc, 'e')
	return enc
}

func bEncodeStr(s string) []byte {
	enc := make([]byte, 0)
	slen := strconv.Itoa(len(s))
	enc = append(enc, []byte(slen)...)
	enc = append(enc, ':')
	enc = append(enc, []byte(s)...)
	return enc
}

func bEncodeList(l []interface{}) ([]byte, error) {
	enc := make([]byte, 0)
	enc = append(enc, 'l')

	for _, e := range l {
		var b []byte
		var err error

		switch v := e.(type) {
		case int, int32, int64:
			num := reflect.ValueOf(v).Int()
			b = bEncodeInt(num)
		case string:
			b = bEncodeStr(v)
		case []interface{}:
			b, err = bEncodeList(v)
		case map[string]interface{}:
			b, err = bEncodeDict(v)
		default:
			err = fmt.Errorf("invalid type %s", v)
		}

		if err != nil {
			return nil, err
		}

		enc = append(enc, b...)
	}
	enc = append(enc, 'e')
	return enc, nil
}

func bEncodeDict(d map[string]interface{}) ([]byte, error) {
	keys := make([]string, 0)
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys) // sort alphabetically

	enc := make([]byte, 0)
	enc = append(enc, 'd')
	for _, k := range keys {

		kbytes := bEncodeStr(k)
		enc = append(enc, kbytes...)

		v := d[k]
		var vbytes []byte
		var err error

		switch val := v.(type) {
		case int, int32, int64:
			num := reflect.ValueOf(val).Int()
			vbytes = bEncodeInt(num)
		case string:
			vbytes = bEncodeStr(val)
		case []interface{}:
			vbytes, err = bEncodeList(val)
		case map[string]interface{}:
			vbytes, err = bEncodeDict(val)
		default:
			err = fmt.Errorf("invalid type : %s", val)
		}

		if err != nil {
			return nil, err
		}
		enc = append(enc, vbytes...)
	}

	enc = append(enc, 'e')
	return enc, nil
}

func findIndex(input []byte, b byte) int {
	for i := 0; i < len(input); i++ {
		if input[i] == b {
			return i
		}
	}
	return -1
}

// input starts with 'i'
// return (integer, dl, error), dl is number of bencoded bytes, i.e 4 for i31e
func bDecodeInt(input []byte) (int64, int, error) {
	e1 := findIndex(input, 'e')
	dec, err := strconv.ParseInt(string(input[1:e1]), 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	dl := e1 + 1
	return dec, dl, nil
}

func bDecodeStr(input []byte) (string, int, error) {
	e1 := findIndex(input, ':')
	slen, err := strconv.Atoi(string(input[:e1]))
	if err != nil {
		fmt.Println(err)
		return "", 0, err
	}
	dl := e1 + slen + 1
	dec := string(input[e1+1 : dl])
	return dec, dl, nil
}

func bDecodeList(input []byte) ([]interface{}, int, error) { // list of interfaces
	result := make([]interface{}, 0)
	input = input[1:]
	dlen := 1

	for len(input) > 0 {
		if input[0] == 'e' {
			dlen++
			break
		}

		var entry interface{}
		var err interface{}
		dl := 0
		switch input[0] {
		case 'i':
			entry, dl, err = bDecodeInt(input)
		case 'l':
			entry, dl, err = bDecodeList(input)
		case 'd':
			entry, dl, err = bDecodeDict(input)
		default:
			entry, dl, err = bDecodeStr(input)
		}

		if err != nil {
			fmt.Println(err)
		}

		result = append(result, entry)
		dlen += dl
		input = input[dl:]
	}

	return result, dlen, nil
}

// keys are bencoded strings
// values can be of any type
func bDecodeDict(input []byte) (map[string]interface{}, int, error) {
	result := make(map[string]interface{}, 0)
	input = input[1:]
	dlen := 1

	for len(input) > 0 {
		if input[0] == 'e' { // end of dictionary
			dlen++
			break
		}

		var key string
		var val interface{}
		var err interface{}
		dl := 0

		// parse key
		key, dl, err = bDecodeStr(input)
		dlen += dl
		input = input[dl:]

		// parse value
		switch input[0] {
		case 'i':
			val, dl, err = bDecodeInt(input)
		case 'l':
			val, dl, err = bDecodeList(input)
		case 'd':
			val, dl, err = bDecodeDict(input)
		default:
			val, dl, err = bDecodeStr(input)
		}

		if err != nil {
			fmt.Println(err)
		}

		result[key] = val
		dlen += dl
		input = input[dl:]
	}

	return result, dlen, nil
}

func BDecode(input []byte) (interface{}, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty input to decode")
	}

	var dec interface{}
	var err error

	switch input[0] {
	case 'i': // integer
		dec, _, err = bDecodeInt(input)
	case 'l': // list
		dec, _, err = bDecodeList(input)
	case 'd': // decode
		dec, _, err = bDecodeDict(input)
	default: // string
		dec, _, err = bDecodeStr(input)
	}

	if err != nil {
		return nil, err
	}
	return dec, nil
}
