package dmsgpty

import (
	"io"
	"net/rpc"

	"github.com/SkycoinProject/dmsg/cipher"
)

type WhitelistClient struct {
	c *rpc.Client
}

func MakeWhitelistClient(conn io.ReadWriteCloser) WhitelistClient {
	return WhitelistClient{c: rpc.NewClient(conn)}
}

// ViewWhitelist obtains the whitelist entries from host.
func (cc WhitelistClient) ViewWhitelist() ([]cipher.PubKey, error) {
	var pks []cipher.PubKey
	err := cc.c.Call(cc.rpcMethod("Whitelist"), &empty, &pks)
	return pks, err
}

// WhitelistAdd adds a whitelist entry to host.
func (cc WhitelistClient) WhitelistAdd(conn io.ReadWriteCloser, pks ...cipher.PubKey) error {
	return cc.c.Call(cc.rpcMethod("WhitelistAdd"), &pks, &empty)
}

// WhitelistRemove removes a whitelist entry from host.
func (cc WhitelistClient) WhitelistRemove(conn io.ReadWriteCloser, pks ...cipher.PubKey) error {
	return cc.c.Call(cc.rpcMethod("WhitelistRemove"), &pks, &empty)
}

func (*WhitelistClient) rpcMethod(m string) string {
	return WhitelistRPCName + "." + m
}
