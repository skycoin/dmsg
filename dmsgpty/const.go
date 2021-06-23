package dmsgpty

import (
	"os"
	"path/filepath"
)

// Constants related to pty.
const (
	PtyRPCName  = "pty"
	PtyURI      = "dmsgpty/pty"
	PtyProxyURI = "dmsgpty/proxy"
)

// Constants related to whitelist.
const (
	WhitelistRPCName = "whitelist"
	WhitelistURI     = "dmsgpty/whitelist"
)

// DefaultCLIAddr gets the default cli address (temp address)
func DefaultCLIAddr() string {
	return filepath.Join(os.TempDir(), "dmsgpty.sock")
}
