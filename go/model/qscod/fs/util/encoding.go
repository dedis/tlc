package util

import (
	"bytes"
	"encoding/gob"

	. "github.com/dedis/tlc/go/model/qscod"
)

// Encode a Value for serialized transmission.
// Currently uses GOB encoding for simplicity,
// but we should change that to something not Go-specific.
func EncodeValue(v Value) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode a Value from its serialized format.
func DecodeValue(b []byte) (v Value, err error) {
	r := bytes.NewReader(b)
	dec := gob.NewDecoder(r)
	err = dec.Decode(&v)
	return
}
