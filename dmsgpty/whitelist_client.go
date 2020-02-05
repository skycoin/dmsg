package dmsgpty

import (
	"io"
	"net/rpc"

	"github.com/SkycoinProject/dmsg/cipher"
)

type WhitelistClient struct {
	c *rpc.Client
}

func NewWhitelistClient(conn io.ReadWriteCloser) *WhitelistClient {
	return &WhitelistClient{c: rpc.NewClient(conn)}
}

// ViewWhitelist obtains the whitelist entries from host.
func (wc WhitelistClient) ViewWhitelist() ([]cipher.PubKey, error) {
	var pks []cipher.PubKey
	err := wc.c.Call(wc.rpcMethod("Whitelist"), &empty, &pks)
	return pks, err
}

// WhitelistAdd adds a whitelist entry to host.
func (wc WhitelistClient) WhitelistAdd(conn io.ReadWriteCloser, pks ...cipher.PubKey) error {
	return wc.c.Call(wc.rpcMethod("WhitelistAdd"), &pks, &empty)
}

// WhitelistRemove removes a whitelist entry from host.
func (wc WhitelistClient) WhitelistRemove(conn io.ReadWriteCloser, pks ...cipher.PubKey) error {
	return wc.c.Call(wc.rpcMethod("WhitelistRemove"), &pks, &empty)
}

func (*WhitelistClient) rpcMethod(m string) string {
	return WhitelistRPCName + "." + m
}
