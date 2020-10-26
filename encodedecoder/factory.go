package encodedecoder

import "fmt"

func New(t Type) EncodeDecoder {
	switch t {
	case TypeGOB:
		return NewGOB()
	case TypeJSON:
		return NewJSON()
	}

	panic(fmt.Errorf("unknown encodedecoder type %s", t))
}
