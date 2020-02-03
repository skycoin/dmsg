package dmsgpty

import (
	"io"
	"net/rpc"
	"os"
	"sync"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
)

// PtyClient represents the client end of a dmsgpty session.
type PtyClient struct {
	log  logrus.FieldLogger
	rpcC *rpc.Client
	done chan struct{}
	once sync.Once
}

func NewPtyClient(conn io.ReadWriteCloser) *PtyClient {
	return &PtyClient{
		log:  logging.MustGetLogger("dmsgpty-client"),
		rpcC: rpc.NewClient(conn),
		done: make(chan struct{}),
	}
}

// Close closes the pty and closes the connection to the remote.
func (sc *PtyClient) Close() error {
	if closed := sc.close(); !closed {
		return nil
	}
	// No need to wait for reply.
	_ = sc.Stop() //nolint:errcheck
	return sc.rpcC.Close()
}

func (sc *PtyClient) close() (closed bool) {
	sc.once.Do(func() {
		close(sc.done)
		closed = true
	})
	return closed
}

// Start starts the pty.
func (sc *PtyClient) Start(name string, arg ...string) error {
	size, err := pty.GetsizeFull(os.Stdin)
	if err != nil {
		sc.log.WithError(err).Warn("failed to obtain terminal size")
		size = nil
	}
	return sc.call("Start", &CommandReq{Name: name, Arg: arg, Size: size}, empty)
}

// Stop stops the pty.
func (sc *PtyClient) Stop() error {
	return sc.call("Stop", empty, empty)
}

// Read reads from the pty.
func (sc *PtyClient) Read(b []byte) (int, error) {
	reqN := len(b)
	var respB []byte
	err := sc.call("Read", &reqN, &respB)
	return copy(b, respB), processRPCError(err)
}

// Write writes to the pty.
func (sc *PtyClient) Write(b []byte) (int, error) {
	var n int
	err := sc.call("Write", &b, &n)
	return n, processRPCError(err)
}

// SetPtySize sets the pty size.
func (sc *PtyClient) SetPtySize(size *pty.Winsize) error {
	return sc.call("SetPtySize", size, empty)
}

func (*PtyClient) rpcMethod(m string) string {
	return PtyRPCName + "." + m
}

func (sc *PtyClient) call(method string, args, reply interface{}) error {
	call := sc.rpcC.Go(sc.rpcMethod(method), args, reply, nil)
	select {
	case <-sc.done:
		return io.ErrClosedPipe // TODO(evanlinjin): Is there a better error to use?
	case <-call.Done:
		return call.Error
	}
}
