package httputil

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestMakeHealthHandler(t *testing.T) {
	const expBase = "health"

	mockEntry := func(code int) HealthGrabberEntry {
		text := http.StatusText(code)
		return HealthGrabberEntry{
			Name: text,
			Grab: func(ctx context.Context) (statusCode int, bodyMsg string) {
				return code, text
			},
		}
	}

	entries := []HealthGrabberEntry{
		mockEntry(http.StatusOK),
		mockEntry(http.StatusNotFound),
	}

	r := chi.NewRouter()
	r.Handle("/"+expBase, MakeHealthHandler(expBase, entries))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	buf := new(bytes.Buffer)
	url := srv.URL + "/" + expBase
	require.NoError(t, CheckHealth(url, buf))

	for i, e := range entries {
		line, err := buf.ReadBytes('\n')
		require.NoError(t, err)

		var code int
		switch i {
		case 0:
			code = http.StatusOK
		case 1:
			code = http.StatusNotFound
		}

		expLine := formatMsg(e.Name, code, e.Name)
		require.Equal(t, expLine, string(line))
	}
}
