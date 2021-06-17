//+build !windows

package dmsgpty

import (
	"github.com/creack/pty"
)

// winSize implements WinSizer for unices (alias for *pty.Winsize)
type winSize pty.Winsize

// NumColumns returns the number of columns
func (w *winSize) NumColumns() uint16 {
	return w.Cols
}

// NumRows returns the number of rows
func (w *winSize) NumRows() uint16 {
	return w.Rows
}

// Width returns the width
func (w *winSize) Width() uint16 {
	return w.X
}

// Height returns the height
func (w *winSize) Height() uint16 {
	return w.Y
}

// PtySize casts the WinSize to *pty.Winsize
func (w *winSize) PtySize() *pty.Winsize {
	return (*pty.Winsize)(w)
}

func newWinSize(size *pty.Winsize) *winSize {
	return (*winSize)(size)
}
