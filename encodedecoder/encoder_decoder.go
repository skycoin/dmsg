package encodedecoder

type EncodeDecoder interface {
	Encode(v interface{}) []byte
	Decode(v interface{}, b []byte) error
}
