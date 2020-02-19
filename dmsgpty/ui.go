package dmsgpty

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
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
		CmdName:  "/bin/bash",
		CmdArgs:  nil,
	}
}

type UI struct {
	conf   UIConfig
	dialer UIDialer
	ptys   map[uuid.UUID]*UISession // upstream host's PTYs.
	mux    sync.RWMutex
}

func NewUI(dialer UIDialer, conf UIConfig) *UI {
	if dialer == nil {
		panic("NewUI: dialer cannot be nil")
	}
	return &UI{
		conf:   conf,
		dialer: dialer,
		ptys:   make(map[uuid.UUID]*UISession),
		mux:    sync.RWMutex{},
	}
}

// ServeMux returns a dmsgpty UI serve mux.
func (ui *UI) UIServeMux(root string) *http.ServeMux {
	root = strings.TrimSuffix(root, "/")

	mux := http.NewServeMux()

	mux.HandleFunc(root,
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleListSessions(ui).ServeHTTP(w, r)
			case http.MethodPost:
				handleNewSession(ui).ServeHTTP(w, r)
			default:
				err := fmt.Errorf("http method %s is invalid for path %s", r.Method, r.URL.EscapedPath())
				writeError(w, r, err, http.StatusMethodNotAllowed)
			}
		})

	mux.HandleFunc(root+"/",
		func(w http.ResponseWriter, r *http.Request) {
			id, err := parseSessionID(r)
			if err != nil {
				writeError(w, r, err, http.StatusBadRequest)
				return
			}
			switch r.Method {
			case http.MethodGet:
				handleOpenSession(ui, id).ServeHTTP(w, r)
			case http.MethodDelete:
				handleDeleteSession(ui, id).ServeHTTP(w, r)
			}
		})

	return mux
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

func (ui *UI) getPty(id uuid.UUID) (*UISession, bool) {
	ui.mux.Lock()
	s, ok := ui.ptys[id]
	ui.mux.Unlock()
	return s, ok
}

func (ui *UI) delPty(id uuid.UUID) {
	ui.mux.Lock()
	_ = ui.ptys[id].Close() //nolint:errcheck
	delete(ui.ptys, id)
	ui.mux.Unlock()
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

	ses := NewUISession(id, ptyC, defaultCacheCap)

	ui.mux.Lock()
	ui.ptys[id] = ses
	ui.mux.Unlock()

	go func() {
		<-ses.WaitClose()
		ui.mux.Lock()
		delete(ui.ptys, id)
		ui.mux.Unlock()
	}()
	return ses, nil
}
