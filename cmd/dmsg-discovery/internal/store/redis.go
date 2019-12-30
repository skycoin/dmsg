package store

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
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
func (r *redisStore) Entry(ctx context.Context, staticPubKey cipher.PubKey) (*disc.Entry, error) {
	payload, err := r.client.Get(staticPubKey.Hex()).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, disc.ErrKeyNotFound
		}
		return nil, disc.ErrUnexpected
	}

	var entry *disc.Entry
	if err := json.Unmarshal(payload, &entry); err != nil {
		log.WithError(err).Warnf("Failed to unmarshal payload %q", payload)
	}
	return entry, nil
}

// Entry implements Storer Entry method for redisdb database
func (r *redisStore) SetEntry(ctx context.Context, entry *disc.Entry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return disc.ErrUnexpected
	}

	err = r.client.Set(entry.Static.Hex(), payload, 0).Err()
	if err != nil {
		return disc.ErrUnexpected
	}

	if entry.Server != nil {
		err = r.client.SAdd("servers", entry.Static.Hex()).Err()
		if err != nil {
			return disc.ErrUnexpected
		}
	}

	return nil
}

// AvailableServers implements Storer AvailableServers method for redisdb database
func (r *redisStore) AvailableServers(ctx context.Context, maxCount int) ([]*disc.Entry, error) {
	entries := make([]*disc.Entry, 0)

	pks, err := r.client.SRandMemberN("servers", int64(maxCount)).Result()
	if err != nil {
		return nil, disc.ErrUnexpected
	}

	if len(pks) == 0 {
		return entries, nil
	}

	payloads, err := r.client.MGet(pks...).Result()
	if err != nil {
		return nil, disc.ErrUnexpected
	}

	for _, payload := range payloads {
		var entry *disc.Entry
		if err := json.Unmarshal([]byte(payload.(string)), &entry); err != nil {
			log.WithError(err).Warnf("Failed to unmarshal payload %s", payload.(string))
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
