package dmsg

import (
	"fmt"
	"sync"
)

// Errors for dial request/response (50-59).
var (
	ErrDialReqInvalidSig       = NewError(50, "dial request has invalid signature", nil)
	ErrDialReqInvalidTimestamp = NewError(51, "dial request timestamp should be higher than last", nil)
	ErrDialReqInvalidSrcPK     = NewError(52, "dial request has invalid source public key", nil)
	ErrDialReqInvalidDstPK     = NewError(53, "dial request has invalid destination public key", nil)
	ErrDialReqInvalidSrcPort   = NewError(54, "dial request has invalid source port", nil)
	ErrDialReqInvalidDstPort   = NewError(55, "dial request has invalid destination port", nil)

	ErrDialRespInvalidSig  = NewError(56, "dial response has invalid signature", nil)
	ErrDialRespInvalidHash = NewError(57, "dial response has invalid hash of associated request", nil)
)

// NetworkErrorOptions provides 'timeout' and 'temporary' options for NetworkError.
type NetworkErrorOptions struct {
	Timeout   bool
	Temporary bool
}

// NetworkError implements 'net.Error'.
type NetworkError struct {
	Err  error
	Opts NetworkErrorOptions
}

func (err NetworkError) Error() string   { return err.Err.Error() }
func (err NetworkError) Timeout() bool   { return err.Opts.Timeout }
func (err NetworkError) Temporary() bool { return err.Opts.Temporary }

var (
	errFmt  = "code %d - %s"
	errMap  = make(map[uint8]error)
	codeMap = make(map[error]uint8)
	errMx   sync.RWMutex
)

// NewError creates a new dmsg error.
// - code '0' represents a miscellaneous error and is not saved in 'errMap'.
// - netOpts is only needed if it needs to implement 'net.Error'.
func NewError(code uint8, msg string, netOpts *NetworkErrorOptions) error {
	// No need to check errMap if code 0.
	if code != 0 {
		errMx.Lock()
		defer errMx.Unlock()
		if _, ok := errMap[code]; ok {
			panic(fmt.Errorf("error of code %d already exists", code))
		}
	}
	err := fmt.Errorf(errFmt, code, msg)
	if netOpts != nil {
		err = &NetworkError{Err: err, Opts: *netOpts}
	}
	// Don't save error if code is '0'.
	if code != 0 {
		errMap[code] = err
		codeMap[err] = code
	}
	return err
}

// ErrorFromCode returns a saved error (if exists) from given error code.
func ErrorFromCode(code uint8) (error, bool) {
	errMx.RLock()
	err, ok := errMap[code]
	errMx.RUnlock()
	return err, ok
}

// CodeFromError returns code from a given error.
func CodeFromError(err error) uint8 {
	errMx.RLock()
	code, ok := codeMap[err]
	errMx.RUnlock()
	if !ok {
		return 0
	}
	return code
}
