package encodedecoder

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigFastest

type jsonED struct{}

func NewJSON() EncodeDecoder {
	return &jsonED{}
}

func (j *jsonED) Encode(v interface{}) []byte {
	bytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (j *jsonED) Decode(v interface{}, b []byte) error {
	return json.Unmarshal(b, v)
}
