package noise

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
)

func readFullPacket(in io.Reader, buf *[]byte, okReads *uint64) (out []byte, err error) {
	defer func() {
		if err != nil {
			fmt.Println("successful reads:", atomic.AddUint64(okReads, 1))
		}
	}()

	// complete prefix bytes
	if r := prefixSize - len(*buf); r > 0 {
		b := make([]byte, r)
		fmt.Println("attempting to read prefix...")
		n, err := io.ReadFull(in, b)
		fmt.Printf("read (prefix): [%d/%d] err(%v) %v\n", n, prefixSize, err, b[:n]) // TODO: remove debug print
		if *buf = append(*buf, b[:n]...); err != nil {
			return nil, err
		}
	}

	// obtain payload size
	paySize := int(binary.BigEndian.Uint32(*buf))

	// complete payload bytes
	if r := prefixSize + paySize - len(*buf); r > 0 {
		b := make([]byte, r)
		fmt.Println("attempting to read payload...")
		n, err := io.ReadFull(in, b)
		fmt.Printf("read (payload): [%d/%d] err(%v) %v\n", n, paySize, err, b[:n]) // TODO: remove debug print
		if *buf = append(*buf, b[:n]...); err != nil {
			return nil, err
		}
	}

	// return success
	out = make([]byte, len(*buf)-prefixSize)
	copy(out, (*buf)[prefixSize:])
	*buf = make([]byte, 0, prefixSize)
	return out, nil
}

func completePacket(in io.Reader, buf *[]byte) error {

	// complete prefix bytes
	if r := prefixSize - len(*buf); r > 0 {
		b := make([]byte, r)
		n, err := io.ReadFull(in, b)
		if *buf = append(*buf, b[:n]...); err != nil {
			return err
		}
	}

	// obtain payload size
	paySize := int(binary.BigEndian.Uint32(*buf))

	// complete payload bytes
	if r := prefixSize + paySize - len(*buf); r > 0 {
		b := make([]byte, r)
		n, err := io.ReadFull(in, b)
		if *buf = append(*buf, b[:n]...); err != nil {
			return err
		}
	}

	return nil
}
