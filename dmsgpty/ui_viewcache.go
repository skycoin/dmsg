package dmsgpty

import (
	"bytes"
	"io"
)

const defaultCacheCap = 4096

func newViewCache(cap int) (cv *viewCache) {
	cv = new(viewCache)
	cv.Grow(cap)
	return cv
}

type viewCache struct {
	bytes.Buffer
}

func (cv *viewCache) Write(p []byte) (int, error) {
	if r := len(p) + cv.Buffer.Len() - cv.Buffer.Cap(); r > 0 {
		if _, err := cv.Buffer.Read(make([]byte, r)); err != nil && err != io.EOF {
			panic(err)
		}
	}
	return cv.Buffer.Write(p)
}