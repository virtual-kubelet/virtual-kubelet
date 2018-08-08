package formatter

import (
	"fmt"
	"sync"

	units "github.com/docker/go-units"
)

const (
	defaultStatsTableFormat = "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}"

	containerHeader = "CONTAINER"
	cpuPercHeader   = "CPU %"
	netIOHeader     = "NET I/O"
	blockIOHeader   = "BLOCK I/O"
	memPercHeader   = "MEM %"
	memUseHeader    = "MEM USAGE / LIMIT"
)

// StatsEntry represents represents the statistics data collected from a container
type StatsEntry struct {
	Container        string
	Name             string
	ID               string
	CPUPercentage    float64
	Memory           float64
	MemoryLimit      float64
	MemoryPercentage float64
	NetworkRx        float64
	NetworkTx        float64
	BlockRead        float64
	BlockWrite       float64
	IsInvalid        bool
}

// ContainerStats represents an entity to store containers statistics synchronously
type ContainerStats struct {
	mutex sync.Mutex
	StatsEntry
	err error
}

// GetError returns the container statistics error.
// This is used to determine whether the statistics are valid or not
func (cs *ContainerStats) GetError() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.err
}

// SetErrorAndReset zeroes all the container statistics and store the error.
// It is used when receiving time out error during statistics collecting to reduce lock overhead
func (cs *ContainerStats) SetErrorAndReset(err error) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.CPUPercentage = 0
	cs.Memory = 0
	cs.MemoryPercentage = 0
	cs.MemoryLimit = 0
	cs.NetworkRx = 0
	cs.NetworkTx = 0
	cs.BlockRead = 0
	cs.BlockWrite = 0
	cs.err = err
	cs.IsInvalid = true
}

// SetError sets container statistics error
func (cs *ContainerStats) SetError(err error) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.err = err
	if err != nil {
		cs.IsInvalid = true
	}
}

// SetStatistics set the container statistics
func (cs *ContainerStats) SetStatistics(s StatsEntry) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	s.Container = cs.Container
	cs.StatsEntry = s
}

// GetStatistics returns container statistics with other meta data such as the container name
func (cs *ContainerStats) GetStatistics() StatsEntry {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.StatsEntry
}

// NewStatsFormat returns a format for rendering an CStatsContext
func NewStatsFormat(source string) Format {
	if source == TableFormatKey {
		return Format(defaultStatsTableFormat)
	}
	return Format(source)
}

// NewContainerStats returns a new ContainerStats entity and sets in it the given name
func NewContainerStats(container string) *ContainerStats {
	return &ContainerStats{
		StatsEntry: StatsEntry{Container: container},
	}
}

// ContainerStatsWrite renders the context for a list of containers statistics
func ContainerStatsWrite(ctx Context, containerStats []StatsEntry) error {
	render := func(format func(subContext subContext) error) error {
		for _, cstats := range containerStats {
			containerStatsCtx := &containerStatsContext{
				s: cstats,
			}
			if err := format(containerStatsCtx); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&containerStatsContext{}, render)
}

type containerStatsContext struct {
	HeaderContext
	s StatsEntry
}

func (c *containerStatsContext) Container() string {
	c.AddHeader(containerHeader)
	return c.s.Container
}

func (c *containerStatsContext) CPUPerc() string {
	c.AddHeader(cpuPercHeader)
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%.2f%%", c.s.CPUPercentage)
}

func (c *containerStatsContext) MemUsage() string {
	header := memUseHeader

	c.AddHeader(header)
	if c.s.IsInvalid {
		return fmt.Sprintf("-- / --")
	}

	return fmt.Sprintf("%s / %s", units.HumanSize(c.s.Memory), units.HumanSize(c.s.MemoryLimit))
}

func (c *containerStatsContext) MemPerc() string {
	header := memPercHeader
	c.AddHeader(header)
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%.2f%%", c.s.MemoryPercentage)
}

func (c *containerStatsContext) NetIO() string {
	c.AddHeader(netIOHeader)
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(c.s.NetworkRx), units.HumanSize(c.s.NetworkTx))
}

func (c *containerStatsContext) BlockIO() string {
	c.AddHeader(blockIOHeader)
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(c.s.BlockRead), units.HumanSize(c.s.BlockWrite))
}
