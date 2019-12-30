package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/gorilla/handlers"

	"github.com/SkycoinProject/dmsg/cipher"
	store2 "github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/httputil"
	"github.com/SkycoinProject/dmsg/metrics"
)

var log = logging.MustGetLogger("dmsg-discovery")

const maxServers = 10

// API represents the api of the messaging-discovery service`
type API struct {
	mux     *http.ServeMux
	store   store2.Storer
	logger  *logging.Logger
	metrics metrics.Recorder
}

// New returns a new API object, which can be started as a server
func New(storer store2.Storer, logger *logging.Logger, metrics metrics.Recorder) *API {
	mux := http.NewServeMux()
	api := &API{
		mux:     mux,
		store:   storer,
		logger:  logger,
		metrics: metrics,
	}

	// routes
	mux.HandleFunc("/messaging-discovery/entry/", api.muxEntry())
	mux.HandleFunc("/messaging-discovery/available_servers", api.getAvailableServer())

	return api
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var h http.Handler
	if a.logger != nil {
		logger := a.logger.WithField("_module", "dmsgdisc_api")
		h = handlers.CustomLoggingHandler(logger.Writer(), a.mux, httputil.WriteLog)
	} else {
		h = a.mux
	}

	metrics.Handler(a.metrics, h).ServeHTTP(w, r)
	w.Header().Set("Content-Type", "application/json")
}

// Start starts the API server
func (a *API) Start(listener net.Listener) error {
	return http.Serve(listener, a)
}

// muxEntry calls either getEntry or setEntry depending on the
// http method used on the endpoint /messaging-discovery/entry/:pk
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
// URI: /messaging-discovery/entry/:pk
// Method: GET
func (a *API) getEntry(w http.ResponseWriter, r *http.Request) {
	staticPK, err := retrievePkFromURL(r.URL)
	if err != nil {
		a.handleError(w, disc.ErrBadInput)
		return
	}

	entry, err := a.store.Entry(r.Context(), staticPK)

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
// URI: /messaging-discovery/entry/
// Method: POST
// Args:
//	json serialized entry object
func (a *API) setEntry(w http.ResponseWriter, r *http.Request) {
	entry := &disc.Entry{}

	err := json.NewDecoder(r.Body).Decode(entry)
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to decode HTTP response body")
		}
	}()

	if err != nil {
		a.handleError(w, disc.ErrUnexpected)
		return
	}

	err = entry.Validate()
	if err != nil {
		a.handleError(w, err)
		return
	}

	err = entry.VerifySignature()
	if err != nil {
		a.handleError(w, disc.ErrUnauthorized)
		return
	}

	// Recover previous entry. If key not found we insert with sequence 0
	// If there was a previous entry we check the new one is a valid iteration
	oldEntry, err := a.store.Entry(r.Context(), entry.Static)
	if err == disc.ErrKeyNotFound {
		if entry.Sequence != 0 {
			a.handleError(w, disc.ErrValidationNonZeroSequence)
			return
		}

		setErr := a.store.SetEntry(r.Context(), entry)
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

	err = oldEntry.ValidateIteration(entry)
	if err != nil {
		a.handleError(w, err)
		return
	}

	err = a.store.SetEntry(r.Context(), entry)
	if err != nil {
		a.handleError(w, err)
		return
	}

	a.writeJSON(w, http.StatusOK, disc.MsgEntryUpdated)
}

// getAvailableServers returns all available server entries as an array of json codified entry objects
// URI: /messaging-discovery/available_servers
// Method: GET
func (a *API) getAvailableServer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := a.store.AvailableServers(r.Context(), maxServers)
		if err != nil {
			a.handleError(w, err)
			return
		}

		a.writeJSON(w, http.StatusOK, entries)
	}
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
		a.logger.Warnf("Failed to encode json response: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(jsonObject)
	if err != nil {
		a.logger.Warnf("Failed to write response: %s", err)
	}
}
