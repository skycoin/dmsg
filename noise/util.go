package noise

import "net"

func isTemporary(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Temporary()
}

func isTimeout(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
