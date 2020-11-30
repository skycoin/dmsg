package store

import (
	"context"
	"time"

	"github.com/go-redis/redis"
	jsoniter "github.com/json-iterator/go"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
)

var json = jsoniter.ConfigFastest

type redisStore struct {
	client  *redis.Client
	timeout time.Duration
}

func newRedis(url, password string, timeout time.Duration) (Storer, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opt.Password = password

	client := redis.NewClient(opt)
	if _, err := client.Ping().Result(); err != nil {
		return nil, err
	}

	return &redisStore{client: client, timeout: timeout}, nil
}

// Entry implements Storer Entry method for redisdb database
func (r *redisStore) Entry(ctx context.Context, staticPubKey cipher.PubKey) (*disc.Entry, error) {
	payload, err := r.client.Get(staticPubKey.Hex()).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, disc.ErrKeyNotFound
		}

		log.WithError(err).WithField("pk", staticPubKey).Errorf("Failed to get entry from redis")
		return nil, disc.ErrUnexpected
	}

	var entry *disc.Entry
	if err := json.Unmarshal(payload, &entry); err != nil {
		log.WithError(err).Warnf("Failed to unmarshal payload %q", payload)
	}

	return entry, nil
}

// Entry implements Storer Entry method for redisdb database
func (r *redisStore) SetEntry(ctx context.Context, entry *disc.Entry, timeout time.Duration) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return disc.ErrUnexpected
	}

	err = r.client.Set(entry.Static.Hex(), payload, timeout).Err()
	if err != nil {
		log.WithError(err).Errorf("Failed to set entry in redis")
		return disc.ErrUnexpected
	}

	if entry.Server != nil {
		err = r.client.SAdd("servers", entry.Static.Hex()).Err()
		if err != nil {
			log.WithError(err).Errorf("Failed to add to servers (SAdd) from redis")
			return disc.ErrUnexpected
		}
	}
	if entry.Client != nil {
		err = r.client.SAdd("clients", entry.Static.Hex()).Err()
		if err != nil {
			log.WithError(err).Errorf("Failed to add to clients (SAdd) from redis")
			return disc.ErrUnexpected
		}
	}

	return nil
}

// AvailableServers implements Storer AvailableServers method for redisdb database
func (r *redisStore) AvailableServers(ctx context.Context, maxCount int) ([]*disc.Entry, error) {
	var entries []*disc.Entry

	pks, err := r.client.SRandMemberN("servers", int64(maxCount)).Result()
	if err != nil {
		log.WithError(err).Errorf("Failed to get servers (SRandMemberN) from redis")
		return nil, disc.ErrUnexpected
	}

	if len(pks) == 0 {
		return entries, nil
	}

	payloads, err := r.client.MGet(pks...).Result()
	if err != nil {
		log.WithError(err).Errorf("Failed to set servers (MGet) from redis")
		return nil, disc.ErrUnexpected
	}

	for _, payload := range payloads {
		// if there's no record for this PK, nil is returned. The below
		// type assertion will panic in this case, so we skip
		if payload == nil {
			continue
		}

		var entry *disc.Entry
		if err := json.Unmarshal([]byte(payload.(string)), &entry); err != nil {
			log.WithError(err).Warnf("Failed to unmarshal payload %s", payload.(string))
			continue
		}

		if entry.Server.AvailableSessions <= 0 {
			log.WithField("server_pk", entry.Static).
				Warn("Server is at max capacity. Skipping...")
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
func (r *redisStore) CountEntries(ctx context.Context) (int64, int64, error) {
	numberOfServers, err := r.client.SCard("servers").Result()
	if err != nil {
		log.WithError(err).Errorf("Failed to get servers count (Scard) from redis")
		return numberOfServers, int64(0), err
	}
	numberOfClients, err := r.client.SCard("clients").Result()
	if err != nil {
		log.WithError(err).Errorf("Failed to get clients count (scard) from redis")
		return numberOfServers, numberOfClients, err
	}

	return numberOfServers, numberOfClients, nil
}
