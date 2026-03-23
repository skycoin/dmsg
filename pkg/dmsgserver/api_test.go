package dmsgserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsg/metrics"
	"github.com/skycoin/dmsg/pkg/dmsgserver"
)

func TestNew_HealthEndpoint(t *testing.T) {
	r := chi.NewRouter()
	log := logging.MustGetLogger("test")
	m := metrics.NewEmpty()

	a := dmsgserver.NewServerAPI(r, log, m)
	require.NotNil(t, a)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestSetDmsgServer(t *testing.T) {
	r := chi.NewRouter()
	log := logging.MustGetLogger("test")
	m := metrics.NewEmpty()

	a := dmsgserver.NewServerAPI(r, log, m)
	require.NotNil(t, a)

	// SetDmsgServer should not panic with a nil server
	assert.NotPanics(t, func() {
		a.SetDmsgServer((*dmsg.Server)(nil))
	})
}
