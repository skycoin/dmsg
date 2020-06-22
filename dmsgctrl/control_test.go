package dmsgctrl

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControl_Ping(t *testing.T) {
	const times = 10

	// arrange
	connA, connB := net.Pipe()
	ctrlA := ControlStream(connA)
	ctrlB := ControlStream(connB)

	t.Cleanup(func() {
		assert.NoError(t, ctrlA.Close())
		assert.NoError(t, ctrlB.Close())
	})

	for i := 0; i < times; i++ {
		durA, err := ctrlA.Ping(context.TODO())
		require.NoError(t, err)
		t.Log(durA)

		durB, err := ctrlB.Ping(context.TODO())
		require.NoError(t, err)
		t.Log(durB)
	}
}

func TestControl_Done(t *testing.T) {
	// arrange
	connA, connB := net.Pipe()
	ctrlA := ControlStream(connA)
	ctrlB := ControlStream(connB)

	// act
	require.NoError(t, ctrlA.Close())
	time.Sleep(time.Millisecond * 200)

	// assert
	assert.True(t, isDone(ctrlA.Done()))
	assert.True(t, isDone(ctrlB.Done()))
}
