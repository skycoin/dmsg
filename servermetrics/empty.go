package servermetrics

import (
	"net/http"
)

// NewEmpty implements Metrics, but does nothing.
func NewEmpty() empty {
	return empty{}
}

type empty struct{}

func (empty) RecordSession(_ DeltaType)                     {}
func (empty) RecordStream(_ DeltaType)                      {}
func (empty) HandleDisc(next http.Handler) http.HandlerFunc { return next.ServeHTTP }
