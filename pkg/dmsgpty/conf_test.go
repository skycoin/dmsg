package dmsgpty_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/skycoin/dmsg/pkg/dmsgpty"
)

func TestParseWindowsConf(t *testing.T) {
	homedrive := "%homedrive%%homepath%\\dmsgpty.sock"
	result := dmsgpty.ParseWindowsEnv(homedrive)
	require.NotEqual(t, "", result)
}
