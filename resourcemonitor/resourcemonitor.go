package resourcemonitor

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/skycoin/skycoin/src/util/logging"
)

const (
	// DefaultInterval is default interval between resource checks.
	DefaultInterval = 1 * time.Minute
	// DefaultCPUThresholdPct is default percentage above which CPU load is considered as high.
	DefaultCPUThresholdPct = 80.0
	// DefaultMemThresholdPct is default percentage above which memory load is considered as high.
	DefaultMemThresholdPct = 80.0
	cpuMeasureInterval     = 1 * time.Second
)

// DefaultOptions define default monitoring options.
var DefaultOptions = Options{
	interval:        DefaultInterval,
	cpuThresholdPct: DefaultCPUThresholdPct,
	memThresholdPct: DefaultMemThresholdPct,
}

// Options define monitoring options.
type Options struct {
	interval        time.Duration
	cpuThresholdPct float64
	memThresholdPct float64
}

// Monitor monitors resources.
type Monitor struct {
	log  *logging.Logger
	opts Options
}

// New returns a new monitor.
func New(log *logging.Logger, opts Options) *Monitor {
	return &Monitor{
		log:  log,
		opts: opts,
	}
}

// StartInBackground starts a goroutine that checks resources.
// It may be canceled by ctx.
func (m *Monitor) StartInBackground(ctx context.Context) {
	ticker := time.NewTicker(m.opts.interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				m.Check()

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Check checks resource consumption.
func (m *Monitor) Check() {
	cpuStat, err := cpu.Percent(cpuMeasureInterval, false)
	if err != nil {
		m.log.WithError(err).Errorf("Failed to check CPU load")
	} else if len(cpuStat) != 0 {
		if stat := cpuStat[0]; stat > m.opts.cpuThresholdPct {
			m.log.Warnf("CPU load is too high: %v", stat)
		}
	}

	memStat, err := mem.VirtualMemory()
	if err != nil {
		m.log.WithError(err).Errorf("Failed to check CPU load")
	} else {
		if used := memStat.UsedPercent; used > m.opts.memThresholdPct {
			m.log.Warnf("Memory load is too high: %v", used)
		}
	}
}
