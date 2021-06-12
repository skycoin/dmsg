//+build windows

package dmsgpty

import (
	"os"

	"github.com/containerd/console"
)

func (ui *UI) uiStartSize(ptyC *PtyClient) error {
	c, err := console.ConsoleFromFile(os.Stdout)
	if err != nil {
		return err
	}
	if err = c.Resize(console.WinSize{
		Height: wsCols,
		Width:  wsRows,
	}); err != nil {
		return err
	}

	return ptyC.StartWithSize(ui.conf.CmdName, ui.conf.CmdArgs, c)
}
