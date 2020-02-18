package dmsgpty

import (
	"github.com/SkycoinProject/dmsg/httputil"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"path"
)

type SessionsJSON struct {
	Sessions []SessionJSON `json:"sessions"`
}

type SessionJSON struct {
	SessionID  uuid.UUID `json:"session_id"`
	SessionURI string    `json:"session_uri"`
}

func makeSessionJSON(id uuid.UUID, reqUrl *url.URL) SessionJSON {
	return SessionJSON{
		SessionID:  id,
		SessionURI: path.Join(reqUrl.EscapedPath(), id.String()),
	}
}

type ErrorJSON struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

func writeError(w http.ResponseWriter, r *http.Request, err error, code int) {
	httputil.WriteJSON(w, r, code, ErrorJSON{
		ErrorCode: code,
		ErrorMsg:  err.Error(),
	})
}

func handleListSessions(ui *UI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := ui.getPtyIDs()
		sessions := make([]SessionJSON, len(ids))
		for i, id := range ids {
			sessions[i] = makeSessionJSON(id, r.URL)
		}
		httputil.WriteJSON(w, r, http.StatusOK, SessionsJSON{
			Sessions: sessions,
		})
	}
}

func handleNewSession(ui *UI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ses, err := ui.newPty()
		if err != nil {
			writeError(w, r, err, http.StatusServiceUnavailable)
			return
		}
		httputil.WriteJSON(w, r, http.StatusOK,
			makeSessionJSON(ses.id, r.URL))
	}
}

func isWebsocket(h http.Header) bool {
	return h.Get("Upgrade") == "websocket"
}