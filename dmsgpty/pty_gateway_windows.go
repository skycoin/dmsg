//+build windows

package dmsgpty

import "golang.org/x/sys/windows"

type winSize windows.Coord

// Width returns the width
func (w *winSize) Width() uint16 {
	return uint16(w.X)
}

// Height returns the height
func (w *winSize) Height() uint16 {
	return uint16(w.Y)
}

// NumColumns is not being used for windows (stub only)
func (w *winSize) NumColumns() uint16 {
	return 0
}

// NumRows is not being used for windows (stub only)
func (w *winSize) NumRows() uint16 {
	return 0
}

func newWinSize(size *windows.Coord) *winSize {
	return (*winSize)(size)
}
