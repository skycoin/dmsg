package store

import (
	"context"

	"github.com/skycoin/skywire/pkg/cipher"
	"github.com/skycoin/skywire/pkg/messaging-discovery/client"
)

// Storer is an interface which allows to implement different kinds of stores
// and choose which one to use in the server
type Storer interface {
	// Entry obtains a single messaging instance entry.
	Entry(ctx context.Context, staticPubKey cipher.PubKey) (*client.Entry, error)

	// SetEntry set's an entry.
	// This is unsafe and does not check signature.
	SetEntry(ctx context.Context, entry *client.Entry) error

	// AvailableServers discovers available messaging servers.
	AvailableServers(ctx context.Context, maxCount int) ([]*client.Entry, error)
}

// NewStore returns an initialized store, name represents which
// store to initialize
func NewStore(name string, urls ...string) (Storer, error) {
	switch name {
	case "mock":
		return newMock(), nil
	default:
		return newRedis(urls[0])
	}
}
