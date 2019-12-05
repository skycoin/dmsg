package netutil

import "io"

// CopyReadWriter copies reads and writes between two connections.
// It returns when a connection returns an error.
func CopyReadWriter(a, b io.ReadWriter) error {
	errCh1 := make(chan error, 1)
	go func() {
		_, err := io.Copy(a, b)
		errCh1 <- err
		close(errCh1)
	}()
	errCh2 := make(chan error, 1)
	go func() {
		_, err := io.Copy(b, a)
		errCh2 <- err
		close(errCh2)
	}()
	select {
	case err := <-errCh1:
		return err
	case err := <-errCh2:
		return err
	}
}
