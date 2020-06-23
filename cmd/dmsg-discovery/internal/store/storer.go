package store

import (
	"context"
	"errors"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

var log = logging.MustGetLogger("store")

var (
	// ErrTooFewArgs is returned on attempt to create a Redis store without passing its URL.
	ErrTooFewArgs = errors.New("too few args")
)

// Storer is an interface which allows to implement different kinds of stores
// and choose which one to use in the server
type Storer interface {
	// Entry obtains a single dmsg instance entry.
	Entry(ctx context.Context, staticPubKey cipher.PubKey) (*disc.Entry, error)

	// SetEntry set's an entry.
	// This is unsafe and does not check signature.
	SetEntry(ctx context.Context, entry *disc.Entry) error

	// AvailableServers discovers available dmsg servers.
	AvailableServers(ctx context.Context, maxCount int) ([]*disc.Entry, error)
}

// Config configures the Store object.
type Config struct {
	URL      string        // database URI
	Password string        // database password
	Timeout  time.Duration // database entry timeout (0 == none)
}

// Config defaults.
const (
	DefaultURL     = "redis://localhost:6379"
	DefaultTimeout = time.Minute
)

// DefaultConfig returns a config with default values.
func DefaultConfig() *Config {
	return &Config{
		URL:     DefaultURL,
		Timeout: DefaultTimeout,
	}
}

// NewStore returns an initialized store, name represents which
// store to initialize
func NewStore(name string, conf *Config) (Storer, error) {
	if conf == nil {
		conf = DefaultConfig()
	}
	switch name {
	case "mock":
		return NewMock(), nil
	case "redis":
		return newRedis(conf.URL, conf.Password, conf.Timeout)
	default:
		return nil, errors.New("no such store type")
	}
}
