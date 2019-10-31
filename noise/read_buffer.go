package noise

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
)

func readFullPacket(in io.Reader, buf *[]byte, okReads *uint64) (out []byte, err error) {
	readI := atomic.AddUint64(okReads, 1)

	fmt.Printf("read[%d] started.\n", readI)
	defer func() {
		if err != nil {
			fmt.Printf("read[%d] finished.\n", readI)
		}
	}()

	// complete prefix bytes
	if r := prefixSize - len(*buf); r > 0 {
		b := make([]byte, r)
		n, err := io.ReadFull(in, b)
		fmt.Printf("read[%d] (prefix): [%d/%d] err(%v) %v\n", readI, n, prefixSize, err, b[:n]) // TODO: remove debug print
		*buf = append(*buf, b[:n]...)
		if err != nil {
			return nil, err
		}
	}

	// obtain payload size
	paySize := int(binary.BigEndian.Uint32(*buf))

	// complete payload bytes
	if r := prefixSize + paySize - len(*buf); r > 0 {
		b := make([]byte, r)
		n, err := io.ReadFull(in, b)
		fmt.Printf("read[%d] (payload): [%d/%d] err(%v) %v\n", readI, n, paySize, err, b[:n]) // TODO: remove debug print
		*buf = append(*buf, b[:n]...)
		if err != nil {
			return nil, err
		}
	}

	// return success
	out = make([]byte, len(*buf)-prefixSize)
	copy(out, (*buf)[prefixSize:])
	*buf = make([]byte, 0, prefixSize)
	return out, nil
}
