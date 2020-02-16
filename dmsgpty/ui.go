package dmsgpty

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"

	"github.com/google/uuid"
)

// UISession represents a UI's session.
type UISession struct {

}

type UI struct {
	ss  map[uuid.UUID]*UISession
	mux sync.RWMutex
}

func (ui *UI) getSessionIDs() []uuid.UUID {
	ui.mux.RLock()
	ids := make([]uuid.UUID, 0, len(ui.ss))
	for id := range ui.ss {
		ids = append(ids, id)
	}
	ui.mux.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i].ID() < ids[j].ID() })
	return ids
}

func handleListSessions(ui *UI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "application/json")

		ids := ui.getSessionIDs()
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(true)
		if err := enc.Encode(ids); err != nil {
			w.WriteHeader(http.StatusInternalServerError)

		}
	}
}