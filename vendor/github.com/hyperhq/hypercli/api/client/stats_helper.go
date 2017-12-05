package client

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/hyperhq/hypercli/cli/command/formatter"
	"golang.org/x/net/context"
)

type stats struct {
	mu sync.Mutex
	cs []*formatter.ContainerStats
}

func (s *stats) add(cs *formatter.ContainerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.Container); !exists {
		s.cs = append(s.cs, cs)
		return true
	}
	return false
}

func (s *stats) remove(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer(id); exists {
		s.cs = append(s.cs[:i], s.cs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *stats) isKnownContainer(cid string) (int, bool) {
	for i, c := range s.cs {
		if c.Container == cid {
			return i, true
		}
	}
	return -1, false
}

func collect(ctx context.Context, s *formatter.ContainerStats, cli *DockerCli, streamStats bool, waitFirst *sync.WaitGroup, c chan events.Message, specified bool) {
	var (
		getFirst       bool
		previousCPU    uint64
		previousSystem uint64
		u              = make(chan error, 1)
		end            = make(chan bool, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	responseBody, err := cli.client.ContainerStats(ctx, s.Container, streamStats)
	if err != nil {
		s.SetError(err)
		return
	}
	defer responseBody.Close()

	dec := json.NewDecoder(responseBody)
	go func() {
		for {
			var (
				v                 *types.StatsJSON
				memPercent        = 0.0
				cpuPercent        = 0.0
				blkRead, blkWrite uint64
				mem               = 0.0
				memLimit          = 0.0
				memPerc           = 0.0
			)

			if err := dec.Decode(&v); err != nil {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), responseBody))
				u <- err
				// TODO: add EOF in hyper.sh
				if err == io.EOF {
					end <- true
					break
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// if mem usage == 0, we think this container is stopped.
			if v.MemoryStats.Usage == 0 {
				if !specified {
					stopEvent := events.Message{
						ID:     s.Container,
						Action: "stop",
					}
					c <- stopEvent
					end <- true
					break
				} else {
					u <- errors.New("This container is stopped.")
					continue
				}
			}

			// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
			// got any data from cgroup
			if v.MemoryStats.Limit != 0 {
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
			}

			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
			cpuPercent = calculateCPUPercent(previousCPU, previousSystem, v)
			blkRead, blkWrite = calculateBlockIO(v.BlkioStats)
			mem = float64(v.MemoryStats.Usage)
			memLimit = float64(v.MemoryStats.Limit)
			memPerc = memPercent
			netRx, netTx := calculateNetwork(v.Networks)

			s.SetStatistics(formatter.StatsEntry{
				CPUPercentage:    cpuPercent,
				Memory:           mem,
				MemoryPercentage: memPerc,
				MemoryLimit:      memLimit,
				NetworkRx:        netRx,
				NetworkTx:        netTx,
				BlockRead:        float64(blkRead),
				BlockWrite:       float64(blkWrite),
			})
			u <- nil
			if !streamStats {
				return
			}
		}
	}()

	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.SetErrorAndReset(errors.New("timeout waiting for stats"))
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case err := <-u:
			if err != nil {
				s.SetError(err)
				continue
			}
			s.SetError(nil)
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case <-end:
			s.SetError(errors.New("This container is stopped."))
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
			break
		}
		if !streamStats {
			return
		}
	}
}

func calculateCPUPercent(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	return float64(v.CPUStats.CPUUsage.TotalUsage) / 100.0
}

func calculateBlockIO(blkio types.BlkioStats) (blkRead uint64, blkWrite uint64) {
	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}
	return
}

func calculateNetwork(network map[string]types.NetworkStats) (float64, float64) {
	var rx, tx float64

	for _, v := range network {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}
