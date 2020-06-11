package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/SkycoinProject/dmsg/disc"
)

var errHandlerTestCases = []struct {
	err        error
	statusCode int
	message    string
}{
	{disc.ErrKeyNotFound, http.StatusNotFound, "entry of public key is not found"},
	{disc.ErrUnexpected, http.StatusInternalServerError, "something unexpected happened"},
	{disc.ErrUnauthorized, http.StatusUnauthorized, "invalid signature"},
	{disc.ErrBadInput, http.StatusBadRequest, "error bad input"},
	{
		disc.NewEntryValidationError("entry Keys is nil"),
		http.StatusUnprocessableEntity,
		"entry validation error: entry Keys is nil",
	},
}

func TestErrorHandler(t *testing.T) {
	for _, tc := range errHandlerTestCases {
		tc := tc
		t.Run(tc.err.Error(), func(t *testing.T) {
			w := httptest.NewRecorder()
			api := New(nil, store.NewMock(), true)
			api.handleError(w, tc.err)

			msg := new(disc.HTTPMessage)
			err := json.NewDecoder(w.Body).Decode(&msg)
			require.NoError(t, err)

			assert.Equal(t, tc.statusCode, msg.Code)
			assert.Equal(t, tc.message, msg.Message)
		})
	}
}
