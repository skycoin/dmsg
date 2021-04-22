package dmsgpty

import (
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/cipher"
)

// Config as
type Config struct {
	DmsgDisc     string         `json:"dmsgdisc"`
	DmsgSessions int            `json:"dmsgsessions"`
	DmsgPort     uint16         `json:"dmsgport"`
	CLINet       string         `json:"clinet"`
	CLIAddr      string         `json:"cliaddr"`
	SK           cipher.SecKey  `json:"-"`
	SKStr        string         `json:"sk"`
	PK           cipher.PubKey  `json:"-"`
	PKStr        string         `json:"pk"`
	WL           cipher.PubKeys `json:"-"`
	WLStr        []string       `json:"wl"`
}

// DefaultConfig is used to populate the config struct with its default values
func DefaultConfig() Config {
	return Config{
		DmsgDisc:     dmsg.DefaultDiscAddr,
		DmsgSessions: dmsg.DefaultMinSessions,
		DmsgPort:     DefaultPort,
		CLINet:       DefaultCLINet,
		CLIAddr:      DefaultCLIAddr,
	}
}
