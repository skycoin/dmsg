package dmsgtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg"
)

func TestEnv(t *testing.T) {

	const timeout = time.Second * 30

	t.Run("startup_shutdown", func(t *testing.T) {
		cases := []struct {
			ServerN     int
			ClientN     int
			MinSessions int
		}{
			{5, 1, 1},
			{5, 3, 1},
			{5, 3, 3},
			{5, 10, 5},
			{5, 10, 10},
		}
		for i, c := range cases {
			env := NewEnv(t, timeout)
			err := env.Startup(c.ServerN, c.ClientN, &dmsg.Config{
				MinSessions: c.MinSessions,
			})
			require.NoError(t, err, i)
			env.Shutdown()
		}
	})

	t.Run("restart_client", func(t *testing.T) {
		env := NewEnv(t, timeout)
		require.NoError(t, env.Startup(3, 1, nil))
		defer env.Shutdown()

		// After closing all clients, n(clients) should be 0.
		require.Len(t, env.AllClients(), 1)
		env.CloseAllClients()
		require.Len(t, env.AllClients(), 0)

		// NewClient should result in n(clients) == 1.
		// Closing the created client should result in n(clients) == 0 after some time.
		client, err := env.NewClient(context.TODO(), nil)
		require.NoError(t, err)
		require.Len(t, env.AllClients(), 1)
		require.NoError(t, client.Close())
		time.Sleep(time.Second)
		require.Len(t, env.AllClients(), 0)
	})
}
