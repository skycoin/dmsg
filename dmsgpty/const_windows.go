//+build windows

package dmsgpty

// Constants related to CLI.
const (
	DefaultCLINet  = "tcp"
	DefaultCLIAddr = "localhost:8083"
)

// Constants related to dmsg.
const (
	DefaultPort = uint16(22)
	DefaultCmd  = "powershell"

)
