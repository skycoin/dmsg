//+build windows

package dmsgpty

import (
	"github.com/containerd/console"
)

// PtyGateway represents a pty gateway, hosted by the pty.SessionServer
type PtyGateway interface {
	Start(req *CommandReq, _ *struct{}) error
	Stop(_, _ *struct{}) error
	Read(reqN *int, respB *[]byte) error
	Write(reqB *[]byte, respN *int) error
	SetPtySize(size console.WinSize, _ *struct{}) error
}

// CommandReq represents a pty command.
type CommandReq struct {
	Name string
	Arg  []string
	Size console.Console
}

// SetPtySize sets the local pty's window size.
func (g *LocalPtyGateway) SetPtySize(size console.WinSize, _ *struct{}) error {
	return g.ses.SetPtySize(size)
}

// SetPtySize sets the remote pty's window size.
func (g *ProxiedPtyGateway) SetPtySize(size console.WinSize, _ *struct{}) error {
	return g.ptyC.SetPtySize(size)
}
