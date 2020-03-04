// +build !no_ci

package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

const (
	redisURL      = "redis://localhost:6379"
	redisPassword = ""
)

func TestRedisStoreClientEntry(t *testing.T) {
	redis, err := newRedis(redisURL, redisPassword)
	require.NoError(t, err)
	require.NoError(t, redis.(*redisStore).client.FlushDB().Err())

	pk, sk := cipher.GenerateKeyPair()
	ctx := context.TODO()

	entry := &disc.Entry{
		Static:    pk,
		Timestamp: time.Now().Unix(),
		Client: &disc.Client{
			DelegatedServers: []cipher.PubKey{pk},
		},
		Version:  "0",
		Sequence: 1,
	}
	require.NoError(t, entry.Sign(sk))

	require.NoError(t, redis.SetEntry(ctx, entry))

	res, err := redis.Entry(ctx, pk)
	require.NoError(t, err)
	assert.Equal(t, entry, res)

	entries, err := redis.AvailableServers(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestRedisStoreServerEntry(t *testing.T) {
	redis, err := newRedis(redisURL, redisPassword)
	require.NoError(t, err)
	require.NoError(t, redis.(*redisStore).client.FlushDB().Err())

	pk, sk := cipher.GenerateKeyPair()
	ctx := context.TODO()

	entry := &disc.Entry{
		Static:    pk,
		Timestamp: time.Now().Unix(),
		Server: &disc.Server{
			Address:     "localhost:8080",
			MaxSessions: 3,
		},
		Version:  "0",
		Sequence: 1,
	}

	require.NoError(t, entry.Sign(sk))

	require.NoError(t, redis.SetEntry(ctx, entry))

	res, err := redis.Entry(ctx, pk)
	require.NoError(t, err)
	assert.Equal(t, entry, res)

	entries, err := redis.AvailableServers(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	require.NoError(t, redis.SetEntry(ctx, entry))

	entries, err = redis.AvailableServers(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
