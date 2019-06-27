package store

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis"
	"github.com/skycoin/skywire/pkg/cipher"

	"github.com/skycoin/skywire/pkg/messaging-discovery/client"
)

type redisStore struct {
	client *redis.Client
}

func newRedis(url string) (Storer, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	_, err = client.Ping().Result()
	if err != nil {
		return nil, err
	}

	return &redisStore{client: client}, nil
}

// Entry implements Storer Entry method for redisdb database
func (r *redisStore) Entry(ctx context.Context, staticPubKey cipher.PubKey) (*client.Entry, error) {
	payload, err := r.client.Get(staticPubKey.Hex()).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, client.ErrKeyNotFound
		}
		return nil, client.ErrUnexpected
	}

	var entry *client.Entry
	json.Unmarshal(payload, &entry) // nolint: errcheck
	return entry, nil
}

// Entry implements Storer Entry method for redisdb database
func (r *redisStore) SetEntry(ctx context.Context, entry *client.Entry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return client.ErrUnexpected
	}

	err = r.client.Set(entry.Static.Hex(), payload, 0).Err()
	if err != nil {
		return client.ErrUnexpected
	}

	if entry.Server != nil {
		err = r.client.SAdd("servers", entry.Static.Hex()).Err()
		if err != nil {
			return client.ErrUnexpected
		}
	}

	return nil
}

// AvailableServers implements Storer AvailableServers method for redisdb database
func (r *redisStore) AvailableServers(ctx context.Context, maxCount int) ([]*client.Entry, error) {
	entries := []*client.Entry{}

	pks, err := r.client.SRandMemberN("servers", int64(maxCount)).Result()
	if err != nil {
		return nil, client.ErrUnexpected
	}

	if len(pks) == 0 {
		return entries, nil
	}

	payloads, err := r.client.MGet(pks...).Result()
	if err != nil {
		return nil, client.ErrUnexpected
	}

	for _, payload := range payloads {
		var entry *client.Entry
		json.Unmarshal([]byte(payload.(string)), &entry) // nolint: errcheck
		entries = append(entries, entry)
	}

	return entries, nil
}
