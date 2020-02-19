package dmsgpty

import (
	"fmt"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/SkycoinProject/dmsg/httputil"

	"net/http"
	"net/url"
	"path"
)

const (
	wsMsgT = websocket.MessageText
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

func handleDeleteSession(ui *UI, id uuid.UUID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ui.delPty(id)
		handleListSessions(ui).ServeHTTP(w, r)
	}
}

func handleOpenSession(ui *UI, id uuid.UUID) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := ui.getPty(id)
		if !ok {
			err := fmt.Errorf("ui session of ID %s does not exist", id)
			writeError(w, r, err, http.StatusNotFound)
			return
		}

		// Serve web page.
		if !isWebsocket(r.Header) {
			if _, err := writeTermHTML(w); err != nil {
				// TODO: log.
			}
			return
		}

		// Serve websocket connection.
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			// TODO: Log.
			return
		}
		defer func() { _ = ws.Close(websocket.StatusNormalClosure, "") }() //nolint:errcheck

		conn := websocket.NetConn(r.Context(), ws, wsMsgT)
		_, _ = session.ServeView(conn) //nolint:errcheck
	}
}

func parseSessionID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(path.Base(r.URL.EscapedPath()))
}

func isWebsocket(h http.Header) bool {
	return h.Get("Upgrade") == "websocket"
}
