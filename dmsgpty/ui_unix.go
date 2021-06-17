//+build !windows

package dmsgpty

import (
	"github.com/creack/pty"
)

func (ui *UI) uiStartSize(ptyC *PtyClient) error {
	size := newWinSize(&pty.Winsize{Rows: wsRows, Cols: wsCols})
	return ptyC.StartWithSize(ui.conf.CmdName, ui.conf.CmdArgs, size)
}
