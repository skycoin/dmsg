package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/discmetrics"
	"github.com/skycoin/dmsg/httputil"
	"github.com/skycoin/dmsg/metricsutil"
)

var log = logging.MustGetLogger("dmsg-discovery")

var json = jsoniter.ConfigFastest

const maxGetAvailableServersResult = 512

// API represents the api of the dmsg-discovery service`
type API struct {
	http.Handler
	metrics                     discmetrics.Metrics
	db                          store.Storer
	reqsInFlightCountMiddleware *metricsutil.RequestsInFlightCountMiddleware
	testMode                    bool
	startedAt                   time.Time
	enableLoadTesting           bool
}

// HealthCheckResponse is struct of /health endpoint
type HealthCheckResponse struct {
	BuildInfo       *buildinfo.Info `json:"build_info"`
	NumberOfServers int64           `json:"servers"`
	StartedAt       time.Time       `json:"started_at,omitempty"`
	Error           string          `json:"error,omitempty"`
}

// New returns a new API object, which can be started as a server
func New(log logrus.FieldLogger, db store.Storer, m discmetrics.Metrics, testMode, enableLoadTesting, enableMetrics bool) *API {
	if log != nil {
		log = logging.MustGetLogger("dmsg_disc")
	}

	if db == nil {
		panic("cannot create new api without a store.Storer")
	}

	r := chi.NewRouter()
	api := &API{
		Handler:                     r,
		metrics:                     m,
		db:                          db,
		testMode:                    testMode,
		startedAt:                   time.Now(),
		enableLoadTesting:           enableLoadTesting,
		reqsInFlightCountMiddleware: metricsutil.NewRequestsInFlightCountMiddleware(),
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	if enableMetrics {
		r.Use(api.reqsInFlightCountMiddleware.Handle)
		r.Use(metricsutil.RequestDurationMiddleware)
	}
	r.Use(httputil.SetLoggerMiddleware(log))

	r.Get("/dmsg-discovery/entry/{pk}", api.getEntry())
	r.Post("/dmsg-discovery/entry/", api.setEntry())
	r.Post("/dmsg-discovery/entry/{pk}", api.setEntry())
	r.Delete("/dmsg-discovery/entry", api.delEntry())
	r.Get("/dmsg-discovery/available_servers", api.getAvailableServers())
	r.Get("/dmsg-discovery/health", api.health())
	r.Get("/health", api.serviceHealth)

	return api
}

func (a *API) log(r *http.Request) logrus.FieldLogger {
	return httputil.GetLogger(r)
}

// RunBackgroundTasks is goroutine which runs in background periodic tasks of dmsg-discovery.
func (a *API) RunBackgroundTasks(ctx context.Context, log logrus.FieldLogger) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	a.updateInternalState(ctx, log)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateInternalState(ctx, log)
		}
	}
}

// getEntry returns the entry associated with the given public key
// URI: /dmsg-discovery/entry/:pk
// Method: GET
func (a *API) getEntry() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		staticPK := cipher.PubKey{}
		if err := staticPK.UnmarshalText([]byte(chi.URLParam(r, "pk"))); err != nil {
			a.handleError(w, r, disc.ErrBadInput)
			return
		}

		entry, err := a.db.Entry(r.Context(), staticPK)

		// If we make sure that every error is handled then we can
		// remove the if and make the entry return the switch default
		if err != nil {
			a.handleError(w, r, err)
			return
		}

		a.writeJSON(w, r, http.StatusOK, entry)
	}
}

// setEntry adds a new entry associated with the given public key
// or updates a previous one if signed by the same instance that
// created the previous one
// URI: /dmsg-discovery/entry/[?timeout={true|false}]
// Method: POST
// Args:
//	json serialized entry object
func (a *API) setEntry() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := r.Body.Close(); err != nil {
				log.WithError(err).Warn("Failed to decode HTTP response body")
			}
		}()

		entryTimeout := time.Duration(0) // no timeout

		entry := new(disc.Entry)
		if err := json.NewDecoder(r.Body).Decode(entry); err != nil {
			a.handleError(w, r, disc.ErrUnexpected)
			return
		}

		if entry.Server != nil && !a.testMode {
			if ok, err := isLoopbackAddr(entry.Server.Address); ok {
				if err != nil {
					a.log(r).Warningf("failed to parse hostname and port: %s", err)
				}

				a.handleError(w, r, disc.ErrValidationServerAddress)
				return
			}
		}

		validateTimestamp := !a.enableLoadTesting
		if err := entry.Validate(validateTimestamp); err != nil {
			a.handleError(w, r, err)
			return
		}

		if !a.enableLoadTesting {
			if err := entry.VerifySignature(); err != nil {
				a.handleError(w, r, disc.ErrUnauthorized)
				return
			}
		}

		// Recover previous entry. If key not found we insert with sequence 0
		// If there was a previous entry we check the new one is a valid iteration
		oldEntry, err := a.db.Entry(r.Context(), entry.Static)
		if err == disc.ErrKeyNotFound {
			setErr := a.db.SetEntry(r.Context(), entry, entryTimeout)
			if setErr != nil {
				a.handleError(w, r, setErr)
				return
			}

			a.writeJSON(w, r, http.StatusOK, disc.MsgEntrySet)

			return
		} else if err != nil {
			a.handleError(w, r, err)
			return
		}

		if !a.enableLoadTesting {
			if err := oldEntry.ValidateIteration(entry); err != nil {
				a.handleError(w, r, err)
				return
			}
		}

		if err := a.db.SetEntry(r.Context(), entry, entryTimeout); err != nil {
			a.handleError(w, r, err)
			return
		}

		a.writeJSON(w, r, http.StatusOK, disc.MsgEntryUpdated)
	}
}

// delEntry deletes the entry associated with the given public key
// URI: /dmsg-discovery/entry
// Method: DELETE
// Args:
//	json serialized entry object
func (a *API) delEntry() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		entry := new(disc.Entry)
		if err := json.NewDecoder(r.Body).Decode(entry); err != nil {
			a.handleError(w, r, disc.ErrUnexpected)
			return
		}

		validateTimestamp := !a.enableLoadTesting
		if err := entry.Validate(validateTimestamp); err != nil {
			a.handleError(w, r, err)
			return
		}

		if !a.enableLoadTesting {
			if err := entry.VerifySignature(); err != nil {
				a.handleError(w, r, disc.ErrUnauthorized)
				return
			}
		}

		err := a.db.DelEntry(r.Context(), entry.Static)

		// If we make sure that every error is handled then we can
		// remove the if and make the entry return the switch default
		if err != nil {
			a.handleError(w, r, err)
			return
		}

		a.writeJSON(w, r, http.StatusOK, disc.MsgEntryDeleted)
	}
}

// getAvailableServers returns all available server entries as an array of json codified entry objects
// URI: /dmsg-discovery/available_servers
// Method: GET
func (a *API) getAvailableServers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := a.db.AvailableServers(r.Context(), maxGetAvailableServersResult)
		if err != nil {
			a.handleError(w, r, err)
			return
		}

		if len(entries) == 0 {
			a.writeJSON(w, r, http.StatusNotFound, disc.HTTPMessage{
				Code:    http.StatusNotFound,
				Message: disc.ErrNoAvailableServers.Error(),
			})

			return
		}

		a.writeJSON(w, r, http.StatusOK, entries)
	}
}

// health returns status of dmsg discovery
// URI: /dmsg-discovery/health
// Method: GET
func (a *API) health() http.HandlerFunc {
	const expBase = "health"
	return httputil.MakeHealthHandler(expBase, nil)
}

func (a *API) serviceHealth(w http.ResponseWriter, r *http.Request) {
	info := buildinfo.Get()
	a.writeJSON(w, r, http.StatusOK, HealthCheckResponse{
		BuildInfo: info,
		StartedAt: a.startedAt,
	})
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

// writeJSON writes a json object on a http.ResponseWriter with the given code.
func (a *API) writeJSON(w http.ResponseWriter, r *http.Request, code int, object interface{}) {
	jsonObject, err := json.Marshal(object)
	if err != nil {
		a.log(r).Warnf("Failed to encode json response: %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_, err = w.Write(jsonObject)
	if err != nil {
		a.log(r).Warnf("Failed to write response: %s", err)
	}
}

func (a *API) updateInternalState(ctx context.Context, logger logrus.FieldLogger) {
	serversCount, clientsCount, err := a.db.CountEntries(ctx)
	if err != nil {
		logger.WithError(err).Errorf("failed to get clients and servers count")
		return
	}

	a.metrics.SetClientsCount(clientsCount)
	a.metrics.SetServersCount(serversCount)
}
