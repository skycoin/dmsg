package dmsgpty

import (
	"io"
)

// PtyRPCName is the universal RPC gateway name.
const PtyRPCName = "pty"

// WhitelistRPCName is the RPC gateway name for 'Whitelist' type requests.
const WhitelistRPCName = "whitelist"

// empty is used for RPC calls.
var empty struct{}

func processRPCError(err error) error {
	if err != nil {
		switch err.Error() {
		case io.EOF.Error():
			return io.EOF
		case io.ErrUnexpectedEOF.Error():
			return io.ErrUnexpectedEOF
		case io.ErrClosedPipe.Error():
			return io.ErrClosedPipe
		case io.ErrNoProgress.Error():
			return io.ErrNoProgress
		case io.ErrShortBuffer.Error():
			return io.ErrShortBuffer
		case io.ErrShortWrite.Error():
			return io.ErrShortWrite
		default:
			return err
		}
	}
	return nil
}
