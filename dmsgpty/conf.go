package dmsgpty

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/skycoin/dmsg"
)

// Config struct is used to read the values from the config.json file
type Config struct {
	DmsgDisc     string   `json:"dmsgdisc"`
	DmsgSessions int      `json:"dmsgsessions"`
	DmsgPort     uint16   `json:"dmsgport"`
	CLINet       string   `json:"clinet"`
	CLIAddr      string   `json:"cliaddr"`
	SK           string   `json:"sk"`
	PK           string   `json:"pk"`
	WL           []string `json:"wl"`
}

// DefaultConfig is used to populate the config struct with its default values
func DefaultConfig() Config {
	return Config{
		DmsgDisc:     dmsg.DefaultDiscAddr,
		DmsgSessions: dmsg.DefaultMinSessions,
		DmsgPort:     DefaultPort,
		CLINet:       DefaultCLINet,
		CLIAddr:      DefaultCLIAddr(),
	}
}

// WriteConfig write the config struct to the provided path
func WriteConfig(conf Config, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(&conf)
}

func findStringsEnclosedBy(str string, sep string, result []string, lastIndex int) ([]string, int) {
	s := strings.Index(str, sep)
	if s == -1 {
		return result, lastIndex
	}
	newS := str[s+len(sep):]
	e := strings.Index(newS, sep)
	if e == -1 {
		return result, lastIndex
	}
	res := newS[:e]
	result = append(result, res)
	lastIndex = s + len(sep) + e
	str = str[lastIndex:]
	return findStringsEnclosedBy(str, sep, result, lastIndex)
}

func ParseWindowsEnv(cliAddr string) string {
	if runtime.GOOS == "windows" {
		var res []string
		var paths []string
		results, lastIndex := findStringsEnclosedBy(cliAddr, "%", res, -1)
		for _, s := range results {
			pth := os.Getenv(strings.ToUpper(s))
			if pth != "" {
				paths = append(paths, pth)
			}
		}
		paths = append(paths, cliAddr[lastIndex:])
		cliAddr = filepath.Join(paths...)
		return cliAddr
	}
	return cliAddr
}
