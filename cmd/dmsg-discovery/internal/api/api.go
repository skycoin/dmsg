package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/gorilla/handlers"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/httputil"
)

var log = logging.MustGetLogger("dmsg-discovery")

const maxGetAvailableServersResult = 512

// API represents the api of the dmsg-discovery service`
type API struct {
	log      logrus.FieldLogger
	db       store.Storer
	testMode bool
	mux      *http.ServeMux
}

// New returns a new API object, which can be started as a server
func New(log logrus.FieldLogger, db store.Storer, testMode bool) *API {
	if log != nil {
		log = logging.MustGetLogger("dmsg_disc")
	}
	if db == nil {
		panic("cannot create new api without a store.Storer")
	}

	mux := http.NewServeMux()
	api := &API{
		log:      log,
		db:       db,
		testMode: testMode,
		mux:      mux,
	}
	mux.HandleFunc("/dmsg-discovery/entry/", api.muxEntry())
	mux.HandleFunc("/dmsg-discovery/available_servers", api.getAvailableServers())
	return api
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := a.log.WithField("_module", "dmsgdisc_api")

	w.Header().Set("Content-Type", "application/json")
	handlers.CustomLoggingHandler(log.Writer(), a.mux, httputil.WriteLog).
		ServeHTTP(w, r)
}

// muxEntry calls either getEntry or setEntry depending on the
// http method used on the endpoint /dmsg-discovery/entry/:pk
func (a *API) muxEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			a.setEntry(w, r)
		default:
			a.getEntry(w, r)
		}
	}
}

// getEntry returns the entry associated with the given public key
// URI: /dmsg-discovery/entry/:pk
// Method: GET
func (a *API) getEntry(w http.ResponseWriter, r *http.Request) {
	staticPK, err := retrievePkFromURL(r.URL)
	if err != nil {
		a.handleError(w, disc.ErrBadInput)
		return
	}

	entry, err := a.db.Entry(r.Context(), staticPK)

	// If we make sure that every error is handled then we can
	// remove the if and make the entry return the switch default
	if err != nil {
		a.handleError(w, err)
		return
	}

	a.writeJSON(w, http.StatusOK, entry)
}

// setEntry adds a new entry associated with the given public key
// or updates a previous one if signed by the same instance that
// created the previous one
// URI: /dmsg-discovery/entry/
// Method: POST
// Args:
//	json serialized entry object
func (a *API) setEntry(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to decode HTTP response body")
		}
	}()

	entry := new(disc.Entry)
	if err := json.NewDecoder(r.Body).Decode(entry); err != nil {
		a.handleError(w, disc.ErrUnexpected)
		return
	}

	if entry.Server != nil && !a.testMode {
		if ok, err := isLoopbackAddr(entry.Server.Address); ok {
			if err != nil && a.log != nil {
				a.log.Warningf("failed to parse hostname and port: %s", err)
			}

			a.handleError(w, disc.ErrValidationServerAddress)
			return
		}
	}

	if err := entry.Validate(); err != nil {
		a.handleError(w, err)
		return
	}

	if err := entry.VerifySignature(); err != nil {
		a.handleError(w, disc.ErrUnauthorized)
		return
	}

	// Recover previous entry. If key not found we insert with sequence 0
	// If there was a previous entry we check the new one is a valid iteration
	oldEntry, err := a.db.Entry(r.Context(), entry.Static)
	if err == disc.ErrKeyNotFound {
		setErr := a.db.SetEntry(r.Context(), entry)
		if setErr != nil {
			a.handleError(w, setErr)
			return
		}

		a.writeJSON(w, http.StatusOK, disc.MsgEntrySet)

		return
	} else if err != nil {
		a.handleError(w, err)
		return
	}

	if err := oldEntry.ValidateIteration(entry); err != nil {
		a.handleError(w, err)
		return
	}

	if err := a.db.SetEntry(r.Context(), entry); err != nil {
		a.handleError(w, err)
		return
	}

	a.writeJSON(w, http.StatusOK, disc.MsgEntryUpdated)
}

// getAvailableServers returns all available server entries as an array of json codified entry objects
// URI: /dmsg-discovery/available_servers
// Method: GET
func (a *API) getAvailableServers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := a.db.AvailableServers(r.Context(), maxGetAvailableServersResult)
		if err != nil {
			a.handleError(w, err)
			return
		}

		if len(entries) == 0 {
			a.writeJSON(w, http.StatusNotFound, disc.HTTPMessage{
				Code:    http.StatusNotFound,
				Message: disc.ErrNoAvailableServers.Error(),
			})

			return
		}

		a.writeJSON(w, http.StatusOK, entries)
	}
}

// isLoopbackAddr checks if string is loopback interface
func isLoopbackAddr(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}

	if host == "" {
		return true, nil
	}

	return net.ParseIP(host).IsLoopback(), nil
}

// retrievePkFromURL returns the id used on endpoints of the form path/:pk
// it doesn't checks if the endpoint has this form and can fail with other
// endpoint forms
func retrievePkFromURL(url *url.URL) (cipher.PubKey, error) {
	splitPath := strings.Split(url.EscapedPath(), "/")
	v := splitPath[len(splitPath)-1]
	pk := cipher.PubKey{}
	err := pk.UnmarshalText([]byte(v))
	return pk, err
}

// writeJSON writes a json object on a http.ResponseWriter with the given code.
func (a *API) writeJSON(w http.ResponseWriter, code int, object interface{}) {
	jsonObject, err := json.Marshal(object)
	if err != nil {
		a.log.Warnf("Failed to encode json response: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_, err = w.Write(jsonObject)
	if err != nil {
		a.log.Warnf("Failed to write response: %s", err)
	}
}
