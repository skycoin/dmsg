package store

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
)

// MockStore implements a storer mock
type MockStore struct {
	mLock       sync.RWMutex
	serversLock sync.RWMutex
	m           map[string][]byte
	servers     map[string][]byte
}

func (ms *MockStore) setEntry(staticPubKey string, payload []byte) {
	ms.mLock.Lock()
	defer ms.mLock.Unlock()

	ms.m[staticPubKey] = payload
}

func (ms *MockStore) entry(staticPubkey string) ([]byte, bool) {
	ms.mLock.RLock()
	defer ms.mLock.RUnlock()

	e, ok := ms.m[staticPubkey]

	return e, ok
}

func (ms *MockStore) setServer(staticPubKey string, payload []byte) {
	ms.serversLock.Lock()
	defer ms.serversLock.Unlock()

	ms.servers[staticPubKey] = payload
}

// NewMock returns a mock storer.
func NewMock() Storer {
	return &MockStore{
		m:       map[string][]byte{},
		servers: map[string][]byte{},
	}
}

// Entry implements Storer Entry method for MockStore
func (ms *MockStore) Entry(ctx context.Context, staticPubKey cipher.PubKey) (*disc.Entry, error) {
	payload, ok := ms.entry(staticPubKey.Hex())
	if !ok {
		return nil, disc.ErrKeyNotFound
	}

	var entry disc.Entry

	// Should not be necessary to check for errors since we control the serialization to JSON`
	err := json.Unmarshal(payload, &entry)
	if err != nil {
		return nil, disc.ErrUnexpected
	}

	err = entry.VerifySignature()
	if err != nil {
		return nil, disc.ErrUnauthorized
	}

	return &entry, nil
}

// SetEntry implements Storer SetEntry method for MockStore
func (ms *MockStore) SetEntry(ctx context.Context, entry *disc.Entry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return disc.ErrUnexpected
	}

	ms.setEntry(entry.Static.Hex(), payload)

	if entry.Server != nil {
		ms.setServer(entry.Static.Hex(), payload)
	}

	return nil
}

// Clear its a mock-only method to clear the mock store data
func (ms *MockStore) Clear() {
	ms.m = map[string][]byte{}
	ms.servers = map[string][]byte{}
}

// AvailableServers implements Storer AvailableServers method for MockStore
func (ms *MockStore) AvailableServers(ctx context.Context, maxCount int) ([]*disc.Entry, error) {
	entries := make([]*disc.Entry, 0)

	ms.serversLock.RLock()
	defer ms.serversLock.RUnlock()

	servers := arrayFromMap(ms.servers)
	for _, entryString := range servers {
		var e disc.Entry

		err := json.Unmarshal(entryString, &e)
		if err != nil {
			return nil, disc.ErrUnexpected
		}

		entries = append(entries, &e)
	}

	return entries, nil
}

func arrayFromMap(m map[string][]byte) [][]byte {
	entries := make([][]byte, 0)

	for _, value := range m {
		buf := make([]byte, len(value))

		copy(buf, value)

		entries = append(entries, buf)
	}

	return entries
}
