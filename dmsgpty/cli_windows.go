//+build windows

package dmsgpty

import (
	"context"
	"sync"
	"time"

	"github.com/ActiveState/termtest/conpty"
)

// ptyResizeLoop informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(ctx context.Context, ptyC *PtyClient) error {
	t := time.NewTicker(2 * time.Second)
	mu := sync.RWMutex{}
	initialSize, err := getSize()
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
			mu.Lock()
			size, err := getSize()
			if err == nil {
				if initialSize != size {
					initialSize = size
					if err = ptyC.SetPtySize(initialSize); err != nil {
						mu.Unlock()
						return err
					}
				}
			}
			mu.Unlock()
		}
	}
}

func (cli *CLI) prepareStdin() (restore func(), err error) {
	return conpty.InitTerminal(true)
}
