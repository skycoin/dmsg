package dmsgpty_test

import (
	"github.com/skycoin/dmsg/dmsgpty"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseWindowsConf(t *testing.T) {
	homedrive := "%homedrive%%homepath%\\dmsgpty.sock"
	result := dmsgpty.ParseWindowsEnv(homedrive)
	require.NotEqual(t, "", result)
	t.Log(result)
}
