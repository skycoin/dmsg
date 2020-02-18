package dmsgpty

import (
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

func TestCacheView_Write(t *testing.T) {
	t.Run("single_write_overflow", func(t *testing.T) {
		const (
			dataLen  = 20
			cacheLen = 10
		)

		data := cipher.RandByte(dataLen)
		cache := newViewCache(cacheLen)

		n, err := cache.Write(data)
		require.NoError(t, err)
		require.Equal(t, dataLen, n)

		b, err := ioutil.ReadAll(cache)
		require.NoError(t, err)

		require.Equal(t, data, b)
	})
	t.Run("multi_write_overflow", func(t *testing.T) {
		const (
			cacheLen = 64
			write1 = 60
			write2 = 5
			writeTotal = write1+write2
		)

		data := cipher.RandByte(writeTotal)
		cache := newViewCache(cacheLen)

		n, err := cache.Write(data[:write1])
		require.NoError(t, err)
		require.Equal(t, write1, n)

		n, err = cache.Write(data[write1:])
		require.NoError(t, err)
		require.Equal(t, write2, n)

		readD, err := ioutil.ReadAll(cache)
		require.NoError(t, err)
		require.Equal(t, data[writeTotal-cacheLen:], readD)
	})
}
