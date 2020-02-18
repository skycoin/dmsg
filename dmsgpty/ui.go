package dmsgpty

import (
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
)

type UIConfig struct {
	TermCols int
	TermRows int
	CmdName  string
	CmdArgs  []string
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		TermCols: 100,
		TermRows: 30,
		CmdName: "/bin/bash",
		CmdArgs: nil,
	}
}

type UI struct {
	conf   UIConfig
	dialer UIDialer
	ptys   map[uuid.UUID]UISession // upstream host's PTYs.
	mux    sync.RWMutex
}

func (ui *UI) getPtyIDs() []uuid.UUID {
	ui.mux.RLock()
	ids := make([]uuid.UUID, 0, len(ui.ptys))
	for id := range ui.ptys {
		ids = append(ids, id)
	}
	ui.mux.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i].ID() < ids[j].ID() })
	return ids
}

func (ui *UI) newPty() (*UISession, error) {
	id := uuid.New()

	hostConn, err := ui.dialer.Dial()
	if err != nil {
		return nil,
			fmt.Errorf("failed to dial to upstream dmsgpty host: %v", err)
	}

	ptyC, err := NewPtyClient(hostConn)
	if err != nil {
		return nil,
			fmt.Errorf("failed to establish pty session with upstream dmsgpty host: %v", err)
	}

	if err := ptyC.StartWithSize(ui.conf.CmdName, ui.conf.CmdArgs, nil); err != nil {
		return nil,
			fmt.Errorf("failed to start pty in upstream dmsgpty host: %v", err)
	}

	ses := UISession{
		id: id,
		hc: ptyC,
		vm: newViewManager(ptyC.log.WithField("ui_id", id.String()), defaultCacheCap),
	}

	ui.mux.Lock()
	ui.ptys[id] = ses
	ui.mux.Unlock()

	return &ses, nil
}

