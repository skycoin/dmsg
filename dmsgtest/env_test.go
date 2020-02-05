package dmsgtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg"
)

func TestNewEnv(t *testing.T) {
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
		t.Run(fmt.Sprint("case_", i), func(t *testing.T) {
			env := NewEnv(t, time.Second*30)
			err := env.Startup(c.ServerN, c.ClientN, &dmsg.Config{
				MinSessions: c.MinSessions,
			})
			require.NoError(t, err)
			env.Shutdown()
		})
	}
}
