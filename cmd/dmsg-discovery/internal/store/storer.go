package store

import (
	"context"
	"errors"

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

// NewStore returns an initialized store, name represents which
// store to initialize
func NewStore(name string, opts ...string) (Storer, error) {
	switch name {
	case "mock":
		return newMock(), nil
	default:
		if len(opts) < 1 {
			return nil, ErrTooFewArgs
		}

		url := opts[0]

		// No password by default.
		password := ""
		if len(opts) > 1 {
			password = opts[1]
		}

		return newRedis(url, password)
	}
}
