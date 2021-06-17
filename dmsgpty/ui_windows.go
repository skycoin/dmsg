//+build windows

package dmsgpty

import "golang.org/x/sys/windows"

func (ui *UI) uiStartSize(ptyC *PtyClient) error {
	coord := &windows.Coord{
		X: wsCols,
		Y: wsRows,
	}

	return ptyC.StartWithSize(ui.conf.CmdName, ui.conf.CmdArgs, newWinSize(coord))
}
