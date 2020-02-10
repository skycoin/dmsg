package cmdutil

import (
	"fmt"
	"strings"
)

// Catch panics on any non-nil error.
func Catch(v ...interface{}) {
	CatchWithMsg("", v...)
}

// CatchWithMsg panics on any non-nil error with the provided message (if any).
func CatchWithMsg(msg string, v ...interface{}) {
	for _, val := range v {
		if err, ok := val.(error); ok && err != nil {
			if msg == "" {
				panic(err)
			}
			msg = strings.TrimSuffix(strings.TrimSpace(msg), ":")
			panic(fmt.Errorf("%s: %v", msg, err))
		}
	}
}
