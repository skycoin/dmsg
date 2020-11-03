package api

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/httputil"
	"github.com/skycoin/skycoin/src/util/logging"
)

// API main object of the server
type API struct {
	numberOfClients      int64
	startedAt            time.Time
	avgPackagesPerMinute uint64
	avgPackagesPerSecond uint64
	dmsgServer           *dmsg.Server
	minuteDecValues      map[*dmsg.SessionCommon]uint64
	minuteEncValues      map[*dmsg.SessionCommon]uint64
	secondDecValues      map[*dmsg.SessionCommon]uint64
	secondEncValues      map[*dmsg.SessionCommon]uint64
	error                string
}

// New returns a new API object, which can be started as a server
func New(r *chi.Mux, log *logging.Logger) *API {
	api := &API{
		startedAt:       time.Now(),
		minuteDecValues: make(map[*dmsg.SessionCommon]uint64),
		minuteEncValues: make(map[*dmsg.SessionCommon]uint64),
		secondDecValues: make(map[*dmsg.SessionCommon]uint64),
		secondEncValues: make(map[*dmsg.SessionCommon]uint64),
	}
	r.Use(httputil.SetLoggerMiddleware(log))
	return api
}

// RunInBackground is function which starts gorutine with periodic tasks of dmsg-server.
func (a *API) RunInBackground(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)
	tickerEverySecond := time.NewTicker(time.Second * 1)
	tickerEveryMinute := time.NewTicker(time.Second * 60)
	defer ticker.Stop()
	defer tickerEverySecond.Stop()
	defer tickerEveryMinute.Stop()
	a.updateInternalState()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateInternalState()
		case <-tickerEveryMinute.C:
			a.updateAverageNumberOfPacketsPerMinute()
		case <-tickerEverySecond.C:
			a.updateAverageNumberOfPacketsPerSecond()
		}
	}
}

// SetDmsgServer saves srv in the API
func (a *API) SetDmsgServer(srv *dmsg.Server) {
	a.dmsgServer = srv
}

// Health serves health page
func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	info := buildinfo.Get()
	a.writeJSON(w, r, http.StatusOK, HealthCheckResponse{
		BuildInfo:            info,
		StartedAt:            a.startedAt,
		NumberOfClients:      a.numberOfClients,
		AvgPackagesPerSecond: a.avgPackagesPerSecond,
		AvgPackagesPerMinute: a.avgPackagesPerMinute,
		Error:                a.error,
	})
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

func (a *API) log(r *http.Request) logrus.FieldLogger {
	return httputil.GetLogger(r)
}

// UpdateInternalState is background function which updates numbers of clients.
func (a *API) updateInternalState() {
	if a.dmsgServer != nil {
		a.numberOfClients = int64(len(a.dmsgServer.GetSessions()))
	}
}

// UpdateAverageNumberOfPacketsPerMinute is function which needs to called every minute.
func (a *API) updateAverageNumberOfPacketsPerMinute() {
	if a.dmsgServer != nil {
		newDecValues, newEncValues, average := calculateThroughput(
			a.dmsgServer.GetSessions(),
			a.minuteDecValues,
			a.minuteEncValues,
		)
		a.minuteDecValues = newDecValues
		a.minuteEncValues = newEncValues
		a.avgPackagesPerMinute = average
	}
}

// UpdateAverageNumberOfPacketsPerSecond is function which needs to called every second.
func (a *API) updateAverageNumberOfPacketsPerSecond() {
	if a.dmsgServer != nil {
		newDecValues, newEncValues, average := calculateThroughput(
			a.dmsgServer.GetSessions(),
			a.secondDecValues,
			a.secondEncValues,
		)
		a.secondDecValues = newDecValues
		a.secondEncValues = newEncValues
		a.avgPackagesPerSecond = average
	}
}
func calculateThroughput(
	sessions map[cipher.PubKey]*dmsg.SessionCommon,
	previousDecValues map[*dmsg.SessionCommon]uint64,
	previousEncValues map[*dmsg.SessionCommon]uint64,
) (
	map[*dmsg.SessionCommon]uint64,
	map[*dmsg.SessionCommon]uint64,
	uint64,
) {

	var average uint64 = math.MaxUint64
	total := big.NewInt(0)
	count := big.NewInt(0)
	bigOne := big.NewInt(1)
	newDecValues := make(map[*dmsg.SessionCommon]uint64)
	newEncValues := make(map[*dmsg.SessionCommon]uint64)
	for _, session := range sessions {
		currentDecValue := session.GetNoise().GetDecNonce()
		currentEncValue := session.GetNoise().GetEncNonce()

		newDecValues[session] = currentDecValue
		newEncValues[session] = currentEncValue

		previousDecValue := previousDecValues[session]
		previousEncValue := previousEncValues[session]
		if currentDecValue != previousDecValue {
			if currentDecValue < previousDecValue {
				// overflow happened
				tmp := new(big.Int).SetUint64(currentDecValue)
				total = total.Add(total, tmp)
				tmp = new(big.Int).SetUint64(math.MaxUint64 - previousDecValue)
				total = total.Add(total, tmp)
			} else {
				tmp := new(big.Int).SetUint64(currentDecValue - previousDecValue)
				total = total.Add(total, tmp)
			}
			count = count.Add(count, bigOne)
		}
		if currentEncValue != previousEncValue {
			if currentEncValue < previousEncValue {
				// overflow happened
				tmp := new(big.Int).SetUint64(currentEncValue)
				total = total.Add(total, tmp)
				tmp = new(big.Int).SetUint64(math.MaxUint64 - previousEncValue)
				total = total.Add(total, tmp)
			} else {
				tmp := new(big.Int).SetUint64(currentEncValue - previousEncValue)
				total = total.Add(total, tmp)
			}
			count = count.Add(count, bigOne)
		}
	}
	if count.Uint64() == uint64(0) {
		average = 0
	} else {
		res := total.Div(total, count)
		if res.IsUint64() {
			average = res.Uint64()
		}
	}
	return newDecValues, newEncValues, average
}
