package cmdutil

import (
	"errors"
	"fmt"
	"log/syslog"
	"strconv"
)

// Syslog associated errors.
var (
	ErrInvalidSyslogString = errors.New("invalid syslog string")
)

type SyslogLvl string

const (
	LvlEmerg   SyslogLvl = "EMERG"
	LvlAlert   SyslogLvl = "ALERT"
	LvlCrit    SyslogLvl = "CRIT"
	LvlErr     SyslogLvl = "ERR"
	LvlWarning SyslogLvl = "WARN"
	LvlNotice  SyslogLvl = "NOTICE"
	LvlInfo    SyslogLvl = "INFO"
	LvlDebug   SyslogLvl = "DEBUG"
)

func init() {
	add := func(lvl SyslogLvl, sp syslog.Priority) { lvlToPri[lvl], priToLvl[sp] = sp, lvl }
	add(LvlEmerg, syslog.LOG_EMERG)
	add(LvlAlert, syslog.LOG_ALERT)
	add(LvlCrit, syslog.LOG_CRIT)
	add(LvlErr, syslog.LOG_ERR)
	add(LvlWarning, syslog.LOG_WARNING)
	add(LvlNotice, syslog.LOG_NOTICE)
	add(LvlInfo, syslog.LOG_INFO)
	add(LvlDebug, syslog.LOG_DEBUG)
}

var (
	lvlToPri = make(map[SyslogLvl]syslog.Priority)
	priToLvl = make(map[syslog.Priority]SyslogLvl)
)

func (l *SyslogLvl) String() string {
	if l == nil {
		return ""
	}
	return string(*l)
}

func (l *SyslogLvl) Set(str string) error {
	if l == nil {
		return nil
	}

	if _, ok := lvlToPri[SyslogLvl(str)]; !ok {
		p, err := strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("%w '%s': %v", ErrInvalidSyslogString, str, err)
		}

		lvl, ok := priToLvl[syslog.Priority(p)]
		if !ok {
			return fmt.Errorf("%w '%s'", ErrInvalidSyslogString, str)
		}

		str = string(lvl)
	}

	*l = SyslogLvl(str)
	return nil
}

func (l *SyslogLvl) Get() interface{} {
	if l == nil {
		return syslog.LOG_EMERG
	}
	return lvlToPri[*l]
}

func (l *SyslogLvl) Type() string {
	return "SyslogLvl"
}
