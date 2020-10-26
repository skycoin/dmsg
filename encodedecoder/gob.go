package encodedecoder

import (
	"bytes"
	"encoding/gob"
)

type gobED struct{}

func NewGOB() EncodeDecoder {
	return &gobED{}
}

func (g *gobED) Encode(v interface{}) []byte {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func (g *gobED) Decode(v interface{}, b []byte) error {
	return gob.NewDecoder(bytes.NewReader(b)).Decode(v)
}
