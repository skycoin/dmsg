package direct

import (
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/disc"
)

const dmsgServerPK = "03f6b0a20be8fe0fd2fd0bd850507cfb10d0eaa37dce5c174654d07db5749a41bf"

// GetServerEntry
func GetServerEntry() *disc.Entry {
	srvPK := cipher.PubKey{}
	_ = srvPK.Set(dmsgServerPK) //nolint:errcheck
	server := &disc.Entry{
		Version: "0.0.1",
		Static:  srvPK,
		Server: &disc.Server{
			Address:           "192.53.115.181:8083",
			AvailableSessions: 1020,
		},
	}
	return server
}

// GetClientEntry
func GetClientEntry(pk cipher.PubKey) *disc.Entry {
	srvPK := cipher.PubKey{}
	_ = srvPK.Set(dmsgServerPK) //nolint:errcheck
	srvPKs := make([]cipher.PubKey, 0)
	srvPKs = append(srvPKs, srvPK)
	client := &disc.Entry{
		Version: "0.0.1",
		Static:  pk,
		Client: &disc.Client{
			DelegatedServers: srvPKs,
		},
	}
	return client
}

// GetAllEntries
func GetAllEntries(pk cipher.PubKey) (entries []*disc.Entry) {

	server := GetServerEntry()
	client := GetClientEntry(pk)
	entries = append(entries, server)
	entries = append(entries, client)
	return entries
}
