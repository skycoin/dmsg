package dmsg

import (
	"context"
	"github.com/SkycoinProject/dmsg/netutil"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/noise"
)

type Stream2 struct {
	lAddr Addr // local address
	rAddr Addr // remote address
	sk  cipher.SecKey     // Local secret key.

	ys  *yamux.Stream     // Underlying yamux stream.
	ns  *noise.ReadWriter // Underlying noise read writer.

	close func() // to be called when closing.
	log   logrus.FieldLogger
}

func NewEphemeralStream(
	ctx context.Context, log logrus.FieldLogger, porter *netutil.Porter, sesNs *noise.Noise, ys *yamux.Stream,
	lPK cipher.PubKey, lSK cipher.SecKey, dst Addr,
) (*Stream2, error) {

	ds := &Stream2{
		lAddr: Addr{PK: lPK},
		rAddr: dst,
		sk: lSK,
		ys: ys,
	}
	lPort, closeDS, err := porter.ReserveEphemeral(ctx, ds)
	if err != nil {
		return nil, err
	}
	ds.lAddr.Port = lPort
	ds.close = closeDS
	ds.log = log.WithField("stream", ds.lAddr.ShortString()+"->"+ds.rAddr.ShortString())

	if err := ds.prepareNoise(sesNs); err != nil {
		return nil, err
	}
	return ds, nil
}

func (ds *Stream2) prepareNoise(sessionNs *noise.Noise) error {
	ns, err := noise.New(noise.HandshakeKK, noise.Config{
		LocalPK:   ds.lAddr.PK,
		LocalSK:   ds.sk,
		RemotePK:  ds.rAddr.PK,
		Initiator: true,
	})
	if err != nil {
		return err
	}

	// Prepare and write request object.
	nsMsg, err := ns.MakeHandshakeMessage()
	if err != nil {
		return err
	}
	req := StreamDialRequest{
		Timestamp: time.Now().UnixNano(),
		SrcAddr:   ds.lAddr,
		DstAddr:   ds.rAddr,
		NoiseMsg:  nsMsg,
	}
	if err := req.Sign(ds.sk); err != nil {
		return err
	}
	if err := writeEncryptedGob(ds.ys, sessionNs, req); err != nil {
		return err
	}

	// Await and read response object.
	var resp DialResponse
	if err := readEncryptedGob(ds.ys, sessionNs, &resp); err != nil {
		return err
	}
	if err := resp.Verify(req.DstAddr.PK, req.Hash()); err != nil {
		return err
	}
	if err := ns.ProcessHandshakeMessage(resp.NoiseMsg); err != nil {
		return err
	}

	// Prepare noise read writer.
	// We do not need to perform a noise handshake here as it is already done.
	ds.ns = noise.NewReadWriter(ds.ys, ns)
	return nil
}

func (ds *Stream2) LocalAddr() net.Addr {
	return ds.lAddr
}

func (ds *Stream2) RemoteAddr() net.Addr {
	return ds.rAddr
}

func (ds *Stream2) Read(b []byte) (int, error) {
	return ds.ns.Read(b)
}

func (ds *Stream2) Write(b []byte) (int, error) {
	return ds.ns.Write(b)
}

func (ds *Stream2) SetDeadline(t time.Time) error {
	return ds.ys.SetDeadline(t)
}

func (ds *Stream2) SetReadDeadline(t time.Time) error {
	return ds.ys.SetReadDeadline(t)
}

func (ds *Stream2) SetWriteDeadline(t time.Time) error {
	return ds.ys.SetWriteDeadline(t)
}

func (ds *Stream2) Close() error {
	ds.close()
	return ds.ys.Close()
}
